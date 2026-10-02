<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import AboutFooter from '$ui/data/AboutFooter.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { loadSourceUrl } from '$lib/instance/source.svelte';
	import { loadSessions } from '$lib/session/session.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import { appUpdate, applyAppUpdate, checkAppUpdate } from '$lib/session/appUpdate.svelte';

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

	// Обновить приложение сразу, не дожидаясь, пока оно уйдёт в фон (C19).
	let checking = $state(false);
	let result = $state<'available' | 'latest' | 'failed' | ''>('');
	async function check() {
		checking = true;
		result = await checkAppUpdate();
		checking = false;
	}

	function goBack() {
		void goUp('/circles');
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
	{#if appUpdate.supported}
		<Hint centered class="mt-8">
			{#if appUpdate.pending}
				<TextButton onclick={() => void applyAppUpdate()}>Обновить до новой версии</TextButton>
			{:else if checking}
				проверяем…
			{:else if result === 'latest'}
				Это последняя версия
			{:else if result === 'failed'}
				Не удалось проверить —
				<TextButton onclick={() => void check()}>ещё раз</TextButton>
			{:else}
				<TextButton onclick={() => void check()}>Проверить обновление</TextButton>
			{/if}
		</Hint>
	{/if}
</FormLayout>
