<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { plural, WORD } from '$lib/format/plural';
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
		// Только число: имена серверов — на экране «Серверы» (C44).
		const sessions = await loadSessions();
		if (sessions.length) subtitle = plural(sessions.length, WORD.server);
	});

	// Обновить приложение сразу, не дожидаясь следующего запуска (C19).
	let checking = $state(false);
	let result = $state<'available' | 'latest' | 'failed' | 'downloading' | ''>('');
	async function check() {
		checking = true;
		result = '';
		result = await checkAppUpdate(() => (result = 'downloading'));
		checking = false;
	}
	// Строка отвечает на месте своей подписью (кадры 7.1–7.5).
	const updateNote = $derived(
		result === 'downloading'
			? 'загружаем новую версию…'
			: checking
				? 'проверяем…'
				: result === 'latest'
					? 'это последняя версия'
					: result === 'failed'
						? 'не удалось проверить — коснитесь ещё раз'
						: 'Wynd обновляется сам'
	);

	function goBack() {
		void goUp('/circles');
	}
</script>

<FormLayout shell app title="Настройки" onback={goBack}>
	<SettingsRow
		class="mt-8"
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
	<SettingsRow icon="day" title="Что нового в Wynd" onclick={() => goto('/settings/news')} />
	{#if appUpdate.supported}
		{#if appUpdate.pending}
			<SettingsRow
				icon="download"
				title="Перейти на новую версию"
				subtitle="Wynd откроется заново"
				chevron={false}
				onclick={() => void applyAppUpdate()}
			/>
		{:else}
			<SettingsRow
				icon="download"
				title="Проверить обновления"
				subtitle={updateNote}
				chevron={false}
				opacity={checking ? 0.55 : undefined}
				onclick={checking ? undefined : () => void check()}
			/>
		{/if}
	{/if}
	<AboutFooter wrap class="mt-28" />
</FormLayout>
