<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import RequisitesCard from '$ui/forms/RequisitesCard.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchPayStatus, formatPayDate } from '$lib/pay/pay';
	import { initSession, loadSessions } from '$lib/session/session.svelte';

	let requisites = $state('');
	let expiresAt = $state<string | null>(null);
	let loading = $state(true);
	let redirecting = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			await initSession();
			const sessions = await loadSessions();
			if (!sessions.length) {
				goto('/');
				return;
			}
			const status = await fetchPayStatus(sessions[0].origin);
			if (!status.has_requisites || status.pending) {
				redirecting = true;
				goto('/circles');
				return;
			}
			requisites = status.requisites;
			expiresAt = status.expires_at;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			if (!redirecting) loading = false;
		}
	});
</script>

<FormLayout app title="Продлить" onback={() => goto('/circles')}>
	{#if loading}
		<Hint>Загрузка…</Hint>
	{:else}
		<Hint>
			{#if expiresAt}
				Подписка до {formatPayDate(expiresAt)}. Круги ещё открыты.
			{:else}
				Круги ещё открыты.
			{/if}
		</Hint>
		<SectionLabel style="margin-top:22px">Куда платить</SectionLabel>
		<RequisitesCard text={requisites} />
		<Button onclick={() => goto('/pay')}>Я оплатил</Button>
		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}
	{/if}
</FormLayout>
