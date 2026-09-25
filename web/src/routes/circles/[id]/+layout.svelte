<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount, setContext } from 'svelte';
	import { resolveCircleOrigin, rememberCircleOrigin, rememberLastCircle } from '$lib/circles/origin';
	import { fetchCircles, loadCirclesCached, ownerNameFromSession } from '$lib/circles/circles';
	import type { CircleListItem } from '$lib/circles/circles';
	import {
		getCircleColor,
		getCircleIdentity,
		setCircleColor,
		setCircleIdentity,
		circleInitial
	} from '$lib/circles/meta';
	import { getSession } from '$lib/idb/db';
	import { CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { fetchCircleDetail } from '$lib/journal/read-cursor';
	import { fetchInvitePeek, isCircleInvitePeek, loadInviteJoinToken } from '$lib/auth/invites';
	import { fetchJoinPreview } from '$lib/circles/settings';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	const circleId = $derived($page.params.id ?? '');
	const onJoinPage = $derived(
		$page.url.pathname === `/circles/${circleId}/join` ||
			$page.url.pathname.endsWith(`/${circleId}/join`)
	);

	let ready = $state(false);
	let denied = $state(false);
	let pendingJoinOnly = $state(false);

	function membershipReadable(status: string | undefined): boolean {
		return status === 'active' || status === 'left_with_access';
	}

	const ctx = $state<CircleContext>({
		origin: '',
		circleId: '',
		name: '',
		color: 'olive',
		colorHex: CIRCLE_COLORS.olive.cssVar,
		identityName: '',
		identityId: '',
		identityInitial: '?',
		avatarBlobId: '',
		avatarUrl: '',
		editWindowSec: undefined,
		lastReadSeq: 0,
		archiveCycle: undefined,
		canWrite: true,
		refresh: async () => {}
	});

	setContext(CIRCLE_CTX, ctx);

	async function loadAvatarUrl(origin: string, blobId?: string) {
		if (blobId) {
			ctx.avatarBlobId = blobId;
			ctx.avatarUrl = await getMediaUrl(origin, blobId);
		} else {
			ctx.avatarBlobId = '';
			ctx.avatarUrl = '';
		}
	}

	async function refreshMeta() {
		if (!ctx.circleId) return;
		try {
			const list = await fetchCircles(ctx.origin);
			const item = list.find((c) => c.id === ctx.circleId);
			if (item) {
				ctx.lastReadSeq = item.last_read_seq;
				if (item.archive_cycle?.active) ctx.archiveCycle = item.archive_cycle;
				ctx.canWrite = item.status === 'active';
			}
			const detail = await fetchCircleDetail(ctx.origin, ctx.circleId);
			if (detail.archive_cycle?.active) ctx.archiveCycle = detail.archive_cycle;
			if (detail.identity_id) ctx.identityId = detail.identity_id;
			ctx.editWindowSec = detail.edit_window_sec;
			const serverName = detail.identity_name?.trim() ?? '';
			if (serverName) {
				ctx.identityName = serverName;
				ctx.identityInitial = circleInitial(serverName);
				await setCircleIdentity(ctx.origin, ctx.circleId, serverName);
			}
			await loadAvatarUrl(ctx.origin, detail.avatar_blob_id);
		} catch {
			/* offline */
		}
	}

	async function loadMeta() {
		pendingJoinOnly = false;
		const resolved = await resolveCircleOrigin(circleId);
		// '' is the same-origin base, not "not found": only null means unknown circle.
		if (resolved === null) {
			goto('/circles');
			return;
		}

		let listItem: CircleListItem | undefined;
		try {
			const list = await fetchCircles(resolved);
			listItem = list.find((c) => c.id === circleId);
		} catch {
			const cached = await loadCirclesCached(resolved);
			listItem = cached.find((c) => c.id === circleId);
		}

		if (!listItem) {
			const inviteToken = loadInviteJoinToken(circleId);
			if (inviteToken) {
				try {
					const peek = await fetchInvitePeek(resolved, inviteToken);
					if (!isCircleInvitePeek(peek)) {
						denied = true;
						return;
					}
					const colorToken = peek.color;
					const color =
						colorToken && colorToken in CIRCLE_COLORS ? colorToken : ('olive' as CircleColor);
					ctx.origin = resolved;
					ctx.circleId = circleId;
					ctx.name = peek.circle_name;
					ctx.color = color;
					ctx.colorHex = CIRCLE_COLORS[color].cssVar;
					ctx.identityName = '';
					ctx.identityId = '';
					ctx.identityInitial = '?';
					ctx.editWindowSec = undefined;
					ctx.lastReadSeq = 0;
					ctx.archiveCycle = undefined;
					ctx.canWrite = true;
					ctx.refresh = loadMeta;
					rememberLastCircle(circleId);
					ready = true;
					return;
				} catch {
					/* fall through */
				}
			}
			try {
				const preview = await fetchJoinPreview(resolved, circleId);
				const colorToken = preview.color;
				const color =
					colorToken && colorToken in CIRCLE_COLORS ? colorToken : ('olive' as CircleColor);
				ctx.origin = resolved;
				ctx.circleId = circleId;
				ctx.name = preview.circle_name;
				ctx.color = color;
				ctx.colorHex = CIRCLE_COLORS[color].cssVar;
				ctx.identityName = '';
				ctx.identityId = '';
				ctx.identityInitial = '?';
				ctx.editWindowSec = undefined;
				ctx.lastReadSeq = 0;
				ctx.archiveCycle = undefined;
				ctx.canWrite = true;
				ctx.refresh = loadMeta;
				rememberLastCircle(circleId);
				rememberCircleOrigin(circleId, resolved);
				pendingJoinOnly = true;
				ready = true;
				if (!onJoinPage) {
					goto(`/circles/${circleId}/join`);
				}
				return;
			} catch {
				/* fall through */
			}
			denied = true;
			ready = true;
			return;
		}

		if (listItem.status === 'gone' || !membershipReadable(listItem.status)) {
			denied = true;
			ready = true;
			return;
		}

		const storedColor = await getCircleColor(resolved, circleId);
		let color: CircleColor;
		const serverColor = listItem.color as CircleColor | undefined;
		if (serverColor && serverColor in CIRCLE_COLORS) {
			color = serverColor;
			await setCircleColor(resolved, circleId, color);
		} else {
			color = storedColor ?? 'olive';
		}
		const session = await getSession(resolved);
		const storedIdentity = await getCircleIdentity(resolved, circleId);

		let archiveCycle = listItem.archive_cycle?.active ? listItem.archive_cycle : undefined;
		let identityId = '';
		let editWindowSec: number | null | undefined;
		let avatarBlobId: string | undefined;
		let serverIdentityName = '';
		try {
			const detail = await fetchCircleDetail(resolved, circleId);
			if (detail.archive_cycle?.active) archiveCycle = detail.archive_cycle;
			identityId = detail.identity_id ?? '';
			editWindowSec = detail.edit_window_sec;
			avatarBlobId = detail.avatar_blob_id;
			serverIdentityName = detail.identity_name?.trim() ?? '';
		} catch {
			/* list banner enough */
		}

		const identityName =
			serverIdentityName ||
			storedIdentity ||
			(session ? ownerNameFromSession(session) : '');
		if (serverIdentityName) {
			await setCircleIdentity(resolved, circleId, serverIdentityName);
		}

		ctx.origin = resolved;
		ctx.circleId = circleId;
		ctx.name = listItem.name;
		ctx.color = color;
		ctx.colorHex = CIRCLE_COLORS[color].cssVar;
		ctx.identityName = identityName;
		ctx.identityId = identityId;
		ctx.identityInitial = circleInitial(identityName);
		ctx.editWindowSec = editWindowSec;
		ctx.lastReadSeq = listItem.last_read_seq;
		ctx.archiveCycle = archiveCycle;
		ctx.canWrite = listItem.status === 'active';
		ctx.refresh = refreshMeta;

		try {
			await loadAvatarUrl(resolved, avatarBlobId);
		} catch {
			ctx.avatarBlobId = '';
			ctx.avatarUrl = '';
		}

		rememberLastCircle(circleId);
		ready = true;
	}

	onMount(() => {
		void loadMeta();
	});
</script>

{#if denied}
	<PlainLayout app>
		<div class="h1s ctr" style="margin-top:80px">Нет доступа</div>
		<div class="hint ctr" style="margin-top:12px">
			<a class="under" href="/circles">К кругам</a>
		</div>
	</PlainLayout>
{:else if ready && !(pendingJoinOnly && !onJoinPage)}
	{@render children()}
{/if}
