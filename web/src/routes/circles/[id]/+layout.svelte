<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { page as pageState } from '$app/state';
	import { setContext, untrack } from 'svelte';
	import { resolveCircleOrigin, rememberCircleOrigin, rememberLastCircle } from '$lib/circles/origin';
	import {
		fetchCircles,
		fetchPendingCircleJoins,
		loadCirclesCached,
		ownerNameFromSession
	} from '$lib/circles/circles';
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
	import Loading from '$ui/Loading.svelte';
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
		sharePlace: true,
		responsesUnread: 0,
		hasOthers: false,
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
			ctx.sharePlace = detail.share_place ?? true;
			ctx.hasOthers = detail.has_others ?? true;
			ctx.responsesUnread = detail.responses_unread ?? 0;
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

	function applyInviteContext(resolved: string, name: string, colorToken: string | undefined) {
		const color =
			colorToken && colorToken in CIRCLE_COLORS ? (colorToken as CircleColor) : ('olive' as CircleColor);
		ctx.origin = resolved;
		ctx.circleId = circleId;
		ctx.name = name;
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
	}

	async function enterAsInvitee(resolved: string): Promise<boolean> {
		const inviteToken = loadInviteJoinToken(circleId);
		if (inviteToken) {
			try {
				const peek = await fetchInvitePeek(resolved, inviteToken);
				if (!isCircleInvitePeek(peek)) return false;
				applyInviteContext(resolved, peek.circle_name, peek.color);
				rememberCircleOrigin(circleId, resolved);
				pendingJoinOnly = true;
				ready = true;
				if (!onJoinPage) goto(`/circles/${circleId}/join`);
				return true;
			} catch {
				/* нет живой ссылки — смотрим отложенное вступление */
			}
		}
		try {
			const preview = await fetchJoinPreview(resolved, circleId);
			applyInviteContext(resolved, preview.circle_name, preview.color);
			rememberCircleOrigin(circleId, resolved);
			pendingJoinOnly = true;
			ready = true;
			if (!onJoinPage) goto(`/circles/${circleId}/join`);
			return true;
		} catch {
			return false;
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

		// Нет читаемого членства — в том числе «gone» после ухода и повторного
		// приглашения. Раньше такой ряд сразу давал «Нет доступа» и не доходил
		// до экрана имени и фото. «Читает, не пишет» тоже проверяем: повторное
		// приглашение иначе открывало хронику и звать было некуда.
		if (listItem?.status === 'left_with_access') {
			const pending = await fetchPendingCircleJoins(resolved).catch(() => [] as string[]);
			if (pending.includes(circleId) && (await enterAsInvitee(resolved))) return;
		}
		if (!listItem || !membershipReadable(listItem.status)) {
			if (await enterAsInvitee(resolved)) return;
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
		let sharePlace = true;
		let hasOthers = true;
		let responsesUnread = 0;
		let avatarBlobId: string | undefined;
		let serverIdentityName = '';
		try {
			const detail = await fetchCircleDetail(resolved, circleId);
			if (detail.archive_cycle?.active) archiveCycle = detail.archive_cycle;
			identityId = detail.identity_id ?? '';
			editWindowSec = detail.edit_window_sec;
			sharePlace = detail.share_place ?? true;
			hasOthers = detail.has_others ?? true;
			responsesUnread = detail.responses_unread ?? 0;
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
		ctx.sharePlace = sharePlace;
		ctx.hasOthers = hasOthers;
		ctx.responsesUnread = responsesUnread;
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

	// Мета круга перечитывается при смене круга, а не только при монтировании:
	// SvelteKit не пересоздаёт макет при смене одного параметра, и переход
	// /circles/A → /circles/B показывал бы шапку и права круга A (план 42, SCR-1).
	let loadedCircle = '';
	$effect(() => {
		const id = circleId;
		if (!id || id === loadedCircle) return;
		loadedCircle = id;
		ready = false;
		denied = false;
		untrack(() => void loadMeta());
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
	<!-- Экраны круга грузят данные при монтировании; смена одного параметра
	     (другая запись, другой день) их не пересоздаёт — пересоздаём по пути.
	     Путь — из $app/state: он меняется в том же такте, что и сам экран.
	     Стор $app/stores отставал, и новый экран монтировался дважды — второй
	     экземпляр терял то, что первый уже забрал (фото из строки ввода). -->
	{#key pageState.url.pathname}
		{@render children()}
	{/key}
{:else}
	<!-- Круг ещё не прочитан (или идёт переход на вступление): раньше здесь
	     был пустой экран. -->
	<PlainLayout app>
		<Loading />
	</PlainLayout>
{/if}
