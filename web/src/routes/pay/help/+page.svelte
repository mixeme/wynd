<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
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

<FormLayout app title="Куда помочь" onback={() => goUp('/circles')}>
	{#if loading}
		<Loading />
	{:else}
		{#if bannerText}
			<Hint>{bannerText}</Hint>
		{/if}
		<SectionLabel class="mt-22">Куда платить</SectionLabel>
		<RequisitesCard text={requisites} />
		<Hint class="mt-22" centered>
			Это добровольно: Wynd работает и без перевода. Спасибо, если поддержите.
		</Hint>
		{#if error}
			<Hint class="mt-12">{error}</Hint>
		{/if}
	{/if}
</FormLayout>
