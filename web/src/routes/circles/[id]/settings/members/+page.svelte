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

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let members = $state<MemberInfo[]>([]);
	let error = $state('');
	let selfId = $state('');
	let isOwner = $state(false);
	let menuMember = $state<MemberInfo | null>(null);
	let transferTarget = $state<MemberInfo | null>(null);
	let transferLoading = $state(false);

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

	async function reload() {
		members = await fetchMembers(circle.origin, circle.circleId);
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

	async function exclude(m: MemberInfo) {
		closeMenu();
		if (!m.account_id) return;
		try {
			await excludeMember(circle.origin, circle.circleId, m.account_id);
			await reload();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const session = await getSession(circle.origin);
			selfId = session?.account_id ?? '';
			members = await fetchMembers(circle.origin, circle.circleId);
			isOwner = members.some((m) => m.is_owner && m.account_id === selfId);
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
		<Hint style="margin:16px">{error}</Hint>
	{:else}
		{#if transferMode}
			<Hint style="margin:16px">Выберите участника, которому передадите круг.</Hint>
		{/if}
		<Label>В круге · {active.length}</Label>
		{#each active as m, i (m.identity_id)}
			<MemberRow
				initial={circleInitial(m.name)}
				name={m.name}
				subtitle={subtitle(m)}
				color={memberColor(i)}
				menu={canManage(m)}
				onmenu={() => openMenu(m)}
				onclick={
					transferMode && !m.is_owner && m.account_id !== selfId
						? () => pickTransfer(m)
						: undefined
				}
				style="padding-top:2px"
			/>
		{/each}
		{#if left.length && !transferMode}
			<Label style="margin-top:18px">Вышли · {left.length}</Label>
			{#each left as m, i (m.identity_id)}
				<MemberRow
					initial={circleInitial(m.name)}
					name={m.name}
					subtitle={subtitle(m)}
					color={memberColor(i)}
					faded
					style="padding-top:2px"
				/>
			{/each}
		{/if}
		{#if !transferMode}
			<Hint style="margin:16px"
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
		<Label style="margin-top:2px">{menuMember.name}</Label>
		<SettingsRow
			title={menuMember.can_settings
				? 'Забрать право менять настройки'
				: 'Дать право менять настройки'}
			chevron={false}
			onclick={() => void toggleSettings(menuMember!)}
		/>
		<SettingsRow
			title="Исключить"
			chevron={false}
			style="color:var(--muted)"
			onclick={() => void exclude(menuMember!)}
		/>
		<Hint style="margin-top:14px"
			>Право выдаёт только владелец.</Hint
		>
	</OverlayLayout>
{/if}

{#if transferTarget}
	<OverlayLayout variant="dialog" label="Передать владение" ondismiss={closeTransfer}>
		<div style="font-size:17px;font-weight:600;margin-bottom:10px">
			Передать «{toAccusativeTitle(circle.name)}» {toDativeName(transferTarget.name)}?
		</div>
		<Hint
			>{transferTarget.name} станет владельцем. Вы останетесь в круге и сможете писать, но
			исключать, передавать владение и удалять круг уже не сможете. Забрать назад можно только
			если {transferTarget.name} передаст вам.</Hint
		>
		<div class="rowin" style="margin:18px 0 0">
			<Button variant="ghost" style="flex:1;margin:0" onclick={closeTransfer}>Отмена</Button>
			<Button style="flex:1;margin:0" loading={transferLoading} onclick={() => void confirmTransfer()}>
				Передать
			</Button>
		</div>
	</OverlayLayout>
{/if}
