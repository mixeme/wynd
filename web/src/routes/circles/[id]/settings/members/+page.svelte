<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
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
	import { formatEntryDate } from '$lib/format/time';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let members = $state<MemberInfo[]>([]);
	let error = $state('');
	let selfId = $state('');
	let isOwner = $state(false);

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

	async function reload() {
		members = await fetchMembers(circle.origin, circle.circleId);
	}

	async function onTransfer(m: MemberInfo) {
		if (!transferMode || m.is_owner || m.account_id === selfId) return;
		if (!confirm(`Передать владение участнику «${m.name}»?`)) return;
		try {
			await transferOwnership(circle.origin, circle.circleId, m.account_id);
			goto(`/circles/${circle.circleId}/settings`);
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function onMenu(m: MemberInfo) {
		if (!canManage(m)) return;
		const action = prompt(
			`«${m.name}»\n1 — Исключить\n2 — ${m.can_settings ? 'Забрать' : 'Дать'} право менять настройки`,
			''
		);
		if (action === '1') {
			if (!confirm(`Исключить «${m.name}» из круга?`)) return;
			try {
				await excludeMember(circle.origin, circle.circleId, m.account_id);
				await reload();
			} catch (err) {
				error = authErrorHint(err);
			}
		} else if (action === '2') {
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
		{#each active as m, i (m.account_id)}
			<MemberRow
				initial={circleInitial(m.name)}
				name={m.name}
				subtitle={subtitle(m)}
				color={CIRCLE_COLORS[CIRCLE_COLOR_ORDER[i % CIRCLE_COLOR_ORDER.length]].cssVar}
				menu={canManage(m)}
				onmenu={() => void onMenu(m)}
				onclick={
					transferMode && !m.is_owner && m.account_id !== selfId
						? () => void onTransfer(m)
						: undefined
				}
				style="padding-top:2px"
			/>
		{/each}
		{#if left.length}
			<Label style="margin-top:18px">Вышли · {left.length}</Label>
			{#each left as m, i (m.account_id)}
				<MemberRow
					initial={circleInitial(m.name)}
					name={m.name}
					subtitle={subtitle(m)}
					color={CIRCLE_COLORS[CIRCLE_COLOR_ORDER[i % CIRCLE_COLOR_ORDER.length]].cssVar}
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
