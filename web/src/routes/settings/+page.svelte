<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Hint from '$ui/forms/Hint.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { appVersion } from '$lib/appinfo';
	import { loadSourceUrl, sourceUrl } from '$lib/instance/source.svelte';
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
	<SettingsRow
		icon="key"
		title="Серверы"
		{subtitle}
		style="margin-top:8px"
		onclick={() => goto('/settings/servers')}
	/>
	<SettingsRow
		icon="bell"
		title="Приложение"
		subtitle="уведомления, место, тема"
		onclick={() => goto('/settings/app')}
	/>
	<Hint centered style="margin-top:44px">
		Wynd {appVersion} · AGPL-3.0<br />
		<a class="under" href={sourceUrl()}>исходный код</a> ·
		<a class="under" href="/THIRD_PARTY_LICENSES.txt">лицензии компонентов</a>
	</Hint>
</FormLayout>
