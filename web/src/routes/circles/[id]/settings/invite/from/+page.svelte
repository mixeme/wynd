<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import SearchGroupHeader from '$ui/data/SearchGroupHeader.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		createMemberInvite,
		fetchInviteCandidates,
		type InviteCandidate,
		type InviteCandidateGroup
	} from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { CIRCLE_COLORS } from '$lib/theme/colors';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const fromCreate = $derived($page.url.searchParams.get('from') === 'create');

	let groups = $state<InviteCandidateGroup[]>([]);
	let error = $state('');
	let loading = $state(true);
	let inviting = $state<string | null>(null);

	function goBack() {
		const q = fromCreate ? '?from=create' : '';
		goto(`/circles/${circle.circleId}/settings/invite${q}`);
	}

	function candidateSubtitle(member: InviteCandidate, group: InviteCandidateGroup): string {
		if (member.membership_status === 'gone' || member.membership_status === 'left_with_access') {
			return `${group.name} · вышла`;
		}
		return group.name;
	}

	function rowValue(member: InviteCandidate): string {
		return member.invited ? 'позвали' : 'позвать';
	}

	async function invite(member: InviteCandidate) {
		if (member.invited || inviting) return;
		inviting = member.account_id;
		error = '';
		try {
			await createMemberInvite(circle.origin, circle.circleId, member.account_id);
			groups = groups.map((g) => ({
				...g,
				members: g.members.map((m) =>
					m.account_id === member.account_id ? { ...m, invited: true } : m
				)
			}));
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			inviting = null;
		}
	}

	onMount(async () => {
		try {
			groups = await fetchInviteCandidates(circle.origin, circle.circleId);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<!-- «Готово» — сразу в круг: «назад» возвращает на 2.7 или в настройки,
     и позвавшему приходилось идти в круг через два экрана. -->
<FormLayout
	app
	color={circle.color}
	title="Из других кругов"
	right="Готово"
	onright={() => goto(`/circles/${circle.circleId}`)}
	onback={goBack}
>
	<Hint style="margin:16px 16px 0">
		Люди из ваших кругов на этом сервере. Группы — круги; имя — как вы их знаете там. В «{circle.name}»
		каждый выберет своё.
	</Hint>
	{#if loading}
		<Loading compact />
	{:else if groups.length === 0}
		<Hint style="margin:16px">Пока никого позвать — нет людей в других ваших кругах.</Hint>
	{:else}
		{#each groups as group (group.id)}
			<SearchGroupHeader
				color={CIRCLE_COLORS[group.color]?.cssVar ?? 'var(--ochre)'}
				name={group.name}
				count={group.count}
				style="margin-top:12px"
			/>
			{#each group.members as member (group.id + member.account_id)}
				{@const faded =
					member.membership_status === 'gone' || member.membership_status === 'left_with_access'}
				<SettingsRow
					title={member.name}
					subtitle={candidateSubtitle(member, group)}
					value={inviting === member.account_id ? '…' : rowValue(member)}
					chevron={false}
					style="padding-top:2px;opacity:{faded ? 0.6 : 1}"
					onclick={member.invited ? undefined : () => void invite(member)}
				/>
			{/each}
		{/each}
		<Hint style="margin:18px 16px 0">Кто уже в «{circle.name}», сюда не попадает. Нажмите на человека — ему придёт приглашение, ссылку отправлять не нужно.</Hint>
	{/if}
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{/if}
</FormLayout>
