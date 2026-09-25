<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchPayStatus, formatPayDate } from '$lib/pay/pay';
	import { initSession, loadSessions } from '$lib/session/session.svelte';

	let requisites = $state('');
	let expiresAt = $state<string | null>(null);
	let loading = $state(true);
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
				goto('/circles');
				return;
			}
			requisites = status.requisites;
			expiresAt = status.expires_at;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
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
		<div class="req">{requisites}</div>
		<Button onclick={() => goto('/pay')}>Я оплатил</Button>
		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}
	{/if}
</FormLayout>

<style>
	.req {
		margin: 0 16px;
		border: 1px solid var(--line);
		background: var(--card);
		border-radius: 12px;
		padding: 12px 14px;
		white-space: pre-wrap;
		font-size: 13.5px;
		line-height: 1.5;
	}
</style>
