<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		inviteRegistrySubtitle,
		inviteRegistryTitle
	} from '$lib/circles/invite-registry';
	import {
		fetchCircleInvites,
		fetchCircleSettings,
		revokeCircleInvite,
		type CircleInvite
	} from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let liveInvites = $state<CircleInvite[]>([]);
	let multiInvitesAllowed = $state(false);
	let error = $state('');
	let loading = $state(true);
	let copiedInviteId = $state('');

	const hasLiveMulti = $derived(liveInvites.some((inv) => inv.kind === 'multi'));

	function goBack() {
		goto(`/circles/${circle.circleId}/settings`);
	}

	function goInvite() {
		goto(`/circles/${circle.circleId}/settings/invite`);
	}

	function inviteUrlFor(token: string): string {
		const base = typeof window !== 'undefined' ? window.location.origin : '';
		return `${base}/invite/${token}`;
	}

	async function loadLiveInvites() {
		liveInvites = await fetchCircleInvites(circle.origin, circle.circleId);
	}

	async function copyInvite(inv: CircleInvite) {
		await navigator.clipboard.writeText(inviteUrlFor(inv.token));
		copiedInviteId = inv.id;
	}

	async function revokeInvite(inv: CircleInvite) {
		try {
			await revokeCircleInvite(circle.origin, circle.circleId, inv.id);
			await loadLiveInvites();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const settings = await fetchCircleSettings(circle.origin, circle.circleId);
			multiInvitesAllowed = settings.invite_kind_default === 'multi';
			await loadLiveInvites();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<FormLayout app color={circle.color} title="Живые ссылки" onback={goBack}>
	{#if loading}
		<Hint style="margin:16px">Загрузка…</Hint>
	{:else if error}
		<Hint style="margin:16px">{error}</Hint>
	{:else if liveInvites.length === 0}
		<Hint style="margin:16px">Живых ссылок нет.</Hint>
		<Button variant="ghost" onclick={goInvite}>Пригласить</Button>
	{:else}
		{#each liveInvites as inv (inv.id)}
			<SettingsRow
				title={inviteRegistryTitle(inv)}
				subtitle={inviteRegistrySubtitle(inv)}
				chevron={false}
				style="padding-top:2px"
			>
				{#snippet control()}
					<span style="display:flex;gap:12px;flex-shrink:0">
						<TextButton onclick={() => void copyInvite(inv)}>
							{copiedInviteId === inv.id ? 'скопировано' : 'скопировать'}
						</TextButton>
						<TextButton onclick={() => void revokeInvite(inv)}>отозвать</TextButton>
					</span>
				{/snippet}
			</SettingsRow>
		{/each}
		{#if !multiInvitesAllowed && hasLiveMulti}
			<Hint style="margin-top:8px"
				>Эту многоразовую успели создать раньше. Новую уже нельзя.</Hint
			>
		{/if}
	{/if}
</FormLayout>
