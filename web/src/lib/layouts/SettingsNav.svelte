<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { navigateBack } from '$lib/navigation/up';
	import { detailEntryOptions } from '$lib/navigation/detailEntry';
	import { plural, WORD } from '$lib/format/plural';
	import AboutFooter from '$ui/data/AboutFooter.svelte';
	import { onMount } from 'svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { loadSourceUrl } from '$lib/instance/source.svelte';
	import { loadSessions } from '$lib/session/session.svelte';
	import { appUpdate, applyAppUpdate, checkAppUpdate } from '$lib/session/appUpdate.svelte';
	import {
		androidUpdate,
		androidUpdateNote,
		applyAndroidUpdate,
		checkAndroidUpdate
	} from '$lib/android/update.svelte';

	// План 76: на широком окне это левая колонка настроек, на телефоне —
	// сам экран «Настройки». Содержимое одно и то же.
	let subtitle = $state('нет серверов');

	const path = $derived(page.url.pathname.replace(/\/+$/, '') || '/');
	const hasDetail = $derived(path !== '/settings');
	const inServers = $derived(path === '/settings/servers' || path.startsWith('/settings/servers/'));
	const inApp = $derived(path === '/settings/app' || path.startsWith('/settings/app/'));
	const inNews = $derived(path === '/settings/news' || path.startsWith('/settings/news/'));

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
		void navigateBack();
	}

	// Раздел из левого списка: первый открывает новую запись истории, смена
	// раздела заменяет — «Назад» не мотает открытые разделы.
	function openSection(target: string) {
		if (path === target) return;
		goto(target, detailEntryOptions(hasDetail));
	}
</script>

<FormLayout shell app title="Настройки" onback={goBack}>
	<SettingsRow
		class="mt-8"
		icon="key"
		title="Серверы"
		{subtitle}
		selected={inServers}
		onclick={() => openSection('/settings/servers')}
	/>
	<SettingsRow
		icon="bell"
		title="Приложение"
		subtitle="уведомления, место, тема"
		selected={inApp}
		onclick={() => openSection('/settings/app')}
	/>
	<SettingsRow
		icon="day"
		title="Что нового в Wynd"
		selected={inNews}
		onclick={() => openSection('/settings/news')}
	/>
	{#if androidUpdate.supported}
		<SettingsRow
			icon="download"
			title={androidUpdate.available
				? `Установить версию ${androidUpdate.nextVersion}`
				: 'Проверить обновления'}
			subtitle={androidUpdateNote()}
			chevron={false}
			opacity={androidUpdate.busy || androidUpdate.installing || androidUpdate.blocked
				? 0.55
				: undefined}
			onclick={androidUpdate.busy || androidUpdate.installing || androidUpdate.blocked
				? undefined
				: () => void (androidUpdate.available ? applyAndroidUpdate() : checkAndroidUpdate())}
		/>
	{:else if appUpdate.supported}
		{#if appUpdate.pending}
			<SettingsRow
				icon="download"
				title="Перейти на новую версию"
				subtitle={appUpdate.applying ? 'перезагружаемся…' : 'Wynd откроется заново'}
				chevron={false}
				opacity={appUpdate.applying ? 0.55 : undefined}
				onclick={appUpdate.applying ? undefined : () => void applyAppUpdate()}
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
	<AboutFooter wrap class="mt-16" />
</FormLayout>
