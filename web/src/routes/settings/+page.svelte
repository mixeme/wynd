<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
	import { plural, WORD } from '$lib/format/plural';
	import AboutFooter from '$ui/data/AboutFooter.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import { interfaceAvailable } from '$lib/android/interface';
	import { displayHost } from '$lib/auth/origin';
	import { loadSourceUrl } from '$lib/instance/source.svelte';
	import { loadSessions } from '$lib/session/session.svelte';
	import { appUpdate, applyAppUpdate, checkAppUpdate } from '$lib/session/appUpdate.svelte';
	import {
		androidUpdate,
		androidUpdateNote,
		applyAndroidUpdate,
		checkAndroidUpdate
	} from '$lib/android/update.svelte';

	let subtitle = $state('нет серверов');
	let nativeInterface = $state(false);
	let sourceHost = $state('');
	let changingServer = $state(false);

	onMount(async () => {
		nativeInterface = interfaceAvailable();
		sourceHost = displayHost(location.origin);
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
	{#if nativeInterface}
		<ServerRow
			card
			name="Сменить сервер"
			subtitle={`приложение открыто с ${sourceHost}`}
			onclick={() => (changingServer = true)}
		/>
	{/if}
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

{#if changingServer}
	<ConfirmDialog
		title="Сменить сервер?"
		top={190}
		confirmLabel="Сменить"
		oncancel={() => (changingServer = false)}
		onconfirm={() => goto('/settings/interface')}
	>
		<Hint class="m-0 mb-8"
			>Этот сервер — то, откуда берётся само приложение: сейчас Wynd загружается с {sourceHost}.</Hint
		>
		<Hint class="m-0 mb-8"
			>Ваши круги от него не зависят: круги с других серверов добавляют в «Серверах», и менять для
			этого ничего не нужно.</Hint
		>
		<Hint class="m-0 mb-8">Менять нужно, если {sourceHost} закрылся или переехал.</Hint>
		<Hint class="m-0"
			>После смены на новом сервере нужно войти заново; записи, которые не успели отправиться, не
			уйдут.</Hint
		>
	</ConfirmDialog>
{/if}
