<script lang="ts">
	import AboutFooter from '$ui/data/AboutFooter.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { loadSourceUrl } from '$lib/instance/source.svelte';
	import { loadSessions } from '$lib/session/session.svelte';

	let subtitle = $state('нет серверов');

	onMount(async () => {
		// Ссылку на исходники даёт сервер, с которого открыт клиент (LIC-2).
		loadSourceUrl();
		const sessions = await loadSessions();
		if (sessions.length === 1) {
			subtitle = sessions[0].name;
		} else if (sessions.length === 2) {
			subtitle = `${sessions[0].name} и ещё один`;
		} else if (sessions.length > 2) {
			subtitle = `${sessions[0].name} и ещё ${sessions.length - 1}`;
		}
	});

	function goBack() {
		void goto('/circles');
	}
</script>

<FormLayout shell app title="Настройки" onback={goBack}>
	<SettingsRow class="mt-8"
		icon="key"
		title="Серверы"
		{subtitle}
		onclick={() => goto('/settings/servers')}
	/>
	<SettingsRow
		icon="bell"
		title="Приложение"
		subtitle="уведомления, место, тема"
		onclick={() => goto('/settings/app')}
	/>
	<AboutFooter wrap class="mt-44" />
</FormLayout>
