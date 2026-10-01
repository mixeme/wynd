<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		excludeMember,
		fetchMembers,
		setMemberCanSettings,
		transferOwnership,
		type MemberInfo
	} from '$lib/circles/settings';
	import { circleInitial } from '$lib/circles/meta';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { getSession } from '$lib/idb/db';
	import { CIRCLE_COLOR_ORDER, CIRCLE_COLORS } from '$lib/theme/colors';
	import { toAccusativeTitle, toDativeName } from '$lib/format/names';
	import { formatEntryDate } from '$lib/format/time';
	import { resolveMediaUrls } from '$lib/media/batch';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let members = $state<MemberInfo[]>([]);
	let error = $state('');
	let selfId = $state('');
	let isOwner = $state(false);
	let menuMember = $state<MemberInfo | null>(null);
	let transferTarget = $state<MemberInfo | null>(null);
	let transferLoading = $state(false);
	let avatarUrls = $state<Record<string, string>>({});
	let excludeTarget = $state<MemberInfo | null>(null);
	let excludeLoading = $state(false);

	const transferMode = $derived($page.url.searchParams.get('transfer') === '1');
	const active = $derived(members.filter((m) => m.status === 'active'));
	const left = $derived(members.filter((m) => m.status !== 'active'));

	function subtitle(m: MemberInfo): string {
		if (m.identity_id === circle.identityId) {
			const parts = ['это вы'];
			const joined = m.joined_at.slice(0, 10);
			if (joined) parts.push(`с ${formatEntryDate(joined)}`);
			return parts.join(' · ');
		}
		const parts: string[] = [];
		if (m.is_owner) parts.push('владелец');
		else if (m.can_settings) parts.push('может менять настройки');
		const joined = m.joined_at.slice(0, 10);
		if (joined) parts.push(`с ${formatEntryDate(joined)}`);
		if (m.status === 'left_with_access') parts.push('читает, не пишет');
		if (m.status === 'gone') parts.push('без доступа');
		return parts.join(' · ');
	}

	function canManage(m: MemberInfo): boolean {
		return isOwner && !transferMode && !m.is_owner && m.identity_id !== circle.identityId;
	}

	function memberColor(index: number): string {
		return CIRCLE_COLORS[CIRCLE_COLOR_ORDER[index % CIRCLE_COLOR_ORDER.length]].cssVar;
	}

	function memberSrc(m: MemberInfo): string | undefined {
		if (m.identity_id === circle.identityId && circle.avatarUrl) return circle.avatarUrl;
		return avatarUrls[m.identity_id];
	}

	async function loadAvatars(list: MemberInfo[]) {
		const byBlob = new Map<string, string[]>();
		for (const m of list) {
			if (!m.avatar_blob_id || avatarUrls[m.identity_id]) continue;
			if (m.identity_id === circle.identityId && circle.avatarUrl) continue;
			byBlob.set(m.avatar_blob_id, [...(byBlob.get(m.avatar_blob_id) ?? []), m.identity_id]);
		}
		const next = { ...avatarUrls };
		await resolveMediaUrls(circle.origin, [...byBlob.keys()], (blobId, url) => {
			for (const identityId of byBlob.get(blobId) ?? []) next[identityId] = url;
			avatarUrls = { ...next };
		});
	}

	async function reload() {
		members = await fetchMembers(circle.origin, circle.circleId);
		await loadAvatars(members);
	}

	function openMenu(m: MemberInfo) {
		if (!canManage(m)) return;
		menuMember = m;
	}

	function closeMenu() {
		menuMember = null;
	}

	function pickTransfer(m: MemberInfo) {
		if (!transferMode || m.is_owner || m.account_id === selfId) return;
		transferTarget = m;
	}

	function closeTransfer() {
		transferTarget = null;
	}

	async function confirmTransfer() {
		// account_id приходит только владельцу и can_settings — без него
		// распоряжаться и нечем.
		if (!transferTarget?.account_id) return;
		transferLoading = true;
		error = '';
		try {
			await transferOwnership(circle.origin, circle.circleId, transferTarget.account_id);
			goto(`/circles/${circle.circleId}/settings`);
		} catch (err) {
			error = authErrorHint(err);
			transferLoading = false;
		}
	}

	async function toggleSettings(m: MemberInfo) {
		closeMenu();
		if (!m.account_id) return;
		try {
			await setMemberCanSettings(
				circle.origin,
				circle.circleId,
				m.account_id,
				!m.can_settings
			);
			await reload();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	function askExclude(m: MemberInfo) {
		closeMenu();
		excludeTarget = m;
	}

	function closeExclude() {
		if (excludeLoading) return;
		excludeTarget = null;
	}

	async function confirmExclude() {
		if (!excludeTarget?.account_id) return;
		excludeLoading = true;
		error = '';
		try {
			await excludeMember(circle.origin, circle.circleId, excludeTarget.account_id);
			excludeTarget = null;
			await reload();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			excludeLoading = false;
		}
	}

	onMount(async () => {
		try {
			const session = await getSession(circle.origin);
			selfId = session?.account_id ?? '';
			members = await fetchMembers(circle.origin, circle.circleId);
			isOwner = members.some((m) => m.is_owner && m.account_id === selfId);
			await loadAvatars(members);
		} catch (err) {
			error = authErrorHint(err);
		}
	});
</script>

<FormLayout
	app
	color={circle.color}
	title={transferMode ? 'Передать владение' : 'Участники'}
	right={String(active.length)}
	onback={() => goto(`/circles/${circle.circleId}/settings`)}
>
	{#if error}
		<Hint class="gutter">{error}</Hint>
	{:else}
		{#if transferMode}
			<Hint class="gutter">Выберите участника, которому передадите круг.</Hint>
		{/if}
		<Label>В круге · {active.length}</Label>
		{#each active as m, i (m.identity_id)}
			<MemberRow
				initial={circleInitial(m.name)}
				name={m.name}
				subtitle={subtitle(m)}
				color={memberColor(i)}
				src={memberSrc(m)}
				menu={canManage(m)}
				onmenu={() => openMenu(m)}
				onclick={
					transferMode && !m.is_owner && m.account_id !== selfId
						? () => pickTransfer(m)
						: undefined
				}
				style={i === 0 ? 'padding-top:2px' : undefined}
			/>
		{/each}
		{#if left.length && !transferMode}
			<Label>Вышли · {left.length}</Label>
			{#each left as m, i (m.identity_id)}
				<MemberRow
					initial={circleInitial(m.name)}
					name={m.name}
					subtitle={subtitle(m)}
					color={memberColor(i)}
					src={memberSrc(m)}
					faded
					style={i === 0 ? 'padding-top:2px' : undefined}
				/>
			{/each}
		{/if}
		{#if !transferMode}
			<Hint class="gutter"
				>Записи вышедших остаются в круге и подписаны тем именем, что было на момент написания.</Hint
			>
			<Button
				variant="ghost"
				style="margin:0 16px"
				onclick={() => goto(`/circles/${circle.circleId}/settings/invite`)}
			>
				Пригласить
			</Button>
		{/if}
	{/if}
</FormLayout>

{#if menuMember}
	<OverlayLayout label={menuMember.name} ondismiss={closeMenu}>
		<Label class="mt-2">{menuMember.name}</Label>
		<SettingsRow
			title={menuMember.can_settings
				? 'Забрать право менять настройки'
				: 'Дать право менять настройки'}
			chevron={false}
			onclick={() => void toggleSettings(menuMember!)}
		/>
		<SettingsRow class="muted"
			title="Исключить"
			chevron={false}
			onclick={() => menuMember && askExclude(menuMember)}
		/>
		<Hint class="mt-14"
			>Право выдаёт только владелец.</Hint
		>
	</OverlayLayout>
{/if}

{#if excludeTarget}
	<OverlayLayout variant="dialog" label="Исключить" ondismiss={closeExclude}>
		<div class="dlgq">
			Исключить из круга: {excludeTarget.name}?
		</div>
		<Hint
			>Записи останутся в круге под этим именем. В журнале будет «покинул круг». Доступа больше не
			будет.</Hint
		>
		<div class="rowin ask">
			<Button variant="ghost" onclick={closeExclude}>Отмена</Button>
			<Button loading={excludeLoading} onclick={() => void confirmExclude()}>Исключить</Button>
		</div>
	</OverlayLayout>
{/if}

{#if transferTarget}
	<OverlayLayout variant="dialog" label="Передать владение" ondismiss={closeTransfer}>
		<div class="dlgq">
			Передать «{toAccusativeTitle(circle.name)}» {toDativeName(transferTarget.name)}?
		</div>
		<Hint
			>{transferTarget.name} станет владельцем. Вы останетесь в круге и сможете писать, но
			исключать, передавать владение и удалять круг уже не сможете. Забрать назад можно только
			если {transferTarget.name} передаст вам.</Hint
		>
		<div class="rowin ask">
			<Button variant="ghost" onclick={closeTransfer}>Отмена</Button>
			<Button loading={transferLoading} onclick={() => void confirmTransfer()}>Передать</Button>
		</div>
	</OverlayLayout>
{/if}
