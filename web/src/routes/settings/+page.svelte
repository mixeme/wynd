<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import AboutFooter from '$ui/data/AboutFooter.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { loadSourceUrl } from '$lib/instance/source.svelte';
	import { loadSessions } from '$lib/session/session.svelte';
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

	// Обновить приложение сразу, не дожидаясь следующего запуска (C19).
	let checking = $state(false);
	let result = $state<'available' | 'latest' | 'failed' | ''>('');
	async function check() {
		checking = true;
		result = await checkAppUpdate();
		checking = false;
	}
	// Строка отвечает на месте своей подписью (кадры 7.23–7.27).
	const updateNote = $derived(
		checking
			? 'проверяем…'
			: result === 'latest'
				? 'это последняя версия'
				: result === 'failed'
					? 'не удалось проверить — коснитесь ещё раз'
					: 'новая версия и так встаёт сама'
	);

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
	<SettingsRow
		icon="day"
		title="Что нового"
		subtitle="что изменилось в Wynd"
		onclick={() => goto('/settings/news')}
	/>
	{#if appUpdate.supported}
		{#if appUpdate.pending}
			<SettingsRow
				icon="download"
				title="Обновить до новой версии"
				subtitle="приложение перезапустится"
				chevron={false}
				onclick={() => void applyAppUpdate()}
			/>
		{:else}
			<SettingsRow
				icon="download"
				title="Проверить обновление"
				subtitle={updateNote}
				chevron={false}
				opacity={checking ? 0.55 : undefined}
				onclick={checking ? undefined : () => void check()}
			/>
		{/if}
	{/if}
	<AboutFooter wrap class="mt-28" />
</FormLayout>
