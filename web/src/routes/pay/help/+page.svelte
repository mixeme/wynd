<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import RequisitesCard from '$ui/forms/RequisitesCard.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchPayStatus } from '$lib/pay/pay';
	import { initSession, loadSessions } from '$lib/session/session.svelte';

	let bannerText = $state('');
	let requisites = $state('');
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
			bannerText = status.banner?.text || '';
			requisites = status.requisites;
			if (!requisites.trim()) goto('/circles');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<FormLayout app title="Куда помочь" onback={() => goto('/circles')}>
	{#if loading}
		<Hint>Загрузка…</Hint>
	{:else}
		{#if bannerText}
			<Hint>{bannerText}</Hint>
		{/if}
		<SectionLabel style="margin-top:22px">Куда платить</SectionLabel>
		<RequisitesCard text={requisites} />
		<Hint centered style="margin-top:22px">
			Это поддержка, не подписка. Круги от перевода не зависят,<br />и заявку отправлять не нужно.
		</Hint>
		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}
	{/if}
</FormLayout>
