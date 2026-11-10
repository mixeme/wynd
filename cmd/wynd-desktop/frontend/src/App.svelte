<script lang="ts">
	import { onMount } from 'svelte';
	import TitleBar from '$ui/workshop/TitleBar.svelte';
	import BackBar from '$ui/chrome/BackBar.svelte';
	import Mark from '$ui/Mark.svelte';
	import Logo from '$ui/Logo.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import {
		applySelfUpdate,
		checkCurrent,
		checkSelfUpdate,
		checkServer,
		currentServer,
		forgetSelected,
		listServers,
		openingReleases,
		runtime,
		selectServer,
		type CheckResult,
		type Update
	} from './api';

	const rt = runtime();
	let maximised = $state(false);
	let address = $state('');
	let error = $state('');
	let checking = $state(false);
	let server = $state<CheckResult | null>(null);
	let servers = $state<CheckResult[]>([]);
	let unavailable = $state(false);
	let loaded = $state(false);
	let showServers = $state(false);
	let adding = $state(false);
	// Новая версия окна (план 63): не молча и не сразу, как у Мастерской.
	let update = $state<Update | null>(null);
	let updateHidden = $state(false);
	let updateBusy = $state(false);
	let updateBad = $state(false);
	let updateError = $state('');
	void checkSelfUpdate()
		.then((u) => (update = u?.available ? u : null))
		.catch(() => undefined);
	async function applyUpdate() {
		if (updateBusy) return;
		updateBusy = true;
		updateError = '';
		try {
			const res = await applySelfUpdate();
			if (res === 'signature') updateBad = true;
			else if (res) updateError = res;
		} finally {
			updateBusy = false;
		}
	}

	onMount(async () => {
		try {
			const saved = await currentServer();
			if (saved) {
				try {
					server = await checkCurrent();
				} catch {
					server = saved;
					unavailable = true;
				}
			}
		} finally {
			loaded = true;
		}
		void syncMaximised();
	});

	async function syncMaximised() {
		maximised = (await rt?.WindowIsMaximised().catch(() => false)) ?? false;
	}

	async function submit() {
		if (!address.trim() || checking) return;
		checking = true;
		error = '';
		try {
			server = await checkServer(address.trim());
			unavailable = false;
			adding = false;
			showServers = false;
		} catch {
			// Причина не различается: опечатка, нет сети, выключенный сервер и
			// чужой сайт выглядят одинаково (ПК4).
			error = 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.';
		} finally {
			checking = false;
		}
	}

	async function retry() {
		if (checking) return;
		checking = true;
		try {
			server = await checkCurrent();
			unavailable = false;
		} catch {
			unavailable = true;
		} finally {
			checking = false;
		}
	}

	async function another() {
		await forgetSelected().catch(() => undefined);
		server = null;
		unavailable = false;
		address = '';
		error = '';
	}

	async function openServers() {
		servers = await listServers().catch(() => []);
		error = '';
		adding = false;
		showServers = true;
	}

	async function choose(origin: string) {
		try {
			const next = await selectServer(origin);
			server = next;
			unavailable = false;
			showServers = false;
		} catch {
			error = 'Не удалось открыть сервер. Попробуйте ещё раз.';
		}
	}

	function openAdd() {
		address = '';
		error = '';
		showServers = false;
		adding = true;
	}

	function closeAdd() {
		adding = false;
		error = '';
		showServers = true;
	}
</script>

<div class="desk-win" class:maximised>
	<TitleBar
		title="Wynd"
		{maximised}
		onminimise={() => rt?.WindowMinimise()}
		ontoggle={() => {
			rt?.WindowToggleMaximise();
			void syncMaximised();
		}}
		onclose={() => rt?.Quit()}
	>
		{#snippet actions()}
			<IconButton name="gear" size="sm" label="Серверы" onclick={() => void openServers()} />
		{/snippet}
	</TitleBar>
	{#if update && !updateHidden}
		<!-- Новая версия: полоса над страницей, как «Вышла версия» у Мастерской. -->
		<div class="desk-update">
			<div class="desk-update-head">Вышла версия {update.version}</div>
			{#if updateBad}
				<div class="desk-update-bad">Подпись не сошлась: обновление не установлено.</div>
			{:else if updateError}
				<div class="desk-update-bad" role="alert">{updateError}</div>
			{/if}
			{#each update.notes ?? [] as note, i (i)}
				<div class="desk-update-note">· {note}</div>
			{/each}
			<div class="desk-update-actions">
				<TextButton disabled={updateBusy} loading={updateBusy} onclick={() => void applyUpdate()}>{updateBusy ? 'Обновляем…' : 'Обновить и перезапустить'}</TextButton>
				<TextButton disabled={updateBusy} onclick={() => (updateHidden = true)}>Позже</TextButton>
				{#if update.attestation}
					<TextButton onclick={() => void openingReleases()}>Что нового</TextButton>
				{/if}
			</div>
		</div>
	{/if}
	{#if !loaded}
		<!-- Первое чтение списка серверов: окно ещё не решает, что показать. -->
	{:else if showServers}
		<!-- ПК7: список серверов и добавление ещё одного. -->
		<div class="page-host">
			<div class="settings-page">
				<BackBar title="Серверы" onback={() => (showServers = false)} />
				<div class="settings-body">
					{#each servers as item, i (item.origin)}
						<ServerRow
							name={item.name}
							subtitle={item.origin.replace(/^https?:\/\//, '')}
							detail={item.origin === server?.origin ? 'Выбран' : 'Открыть'}
							variant={item.origin === server?.origin ? 'ok' : 'select'}
							card
							top={i === 0 ? 10 : 26}
							onclick={() => void choose(item.origin)}
						/>
					{/each}
					{#if error}
						<Hint class="mt-16">{error}</Hint>
					{/if}
					<Hint class="mt-24">
						Общего профиля на все серверы нет: каждый сервер — отдельное место, даже с той же
						почтой.
					</Hint>
					<Button onclick={openAdd}>Добавить сервер</Button>
				</div>
			</div>
		</div>
	{:else if adding}
		<!-- Добавление ещё одного сервера: тот же адрес, что на ПК1. -->
		<div class="desk-content">
			<div class="welcome">
				<div class="welcome-mark"><Mark /></div>
				<h1>Добавить сервер</h1>
				<Label>Адрес сервера</Label>
				<Input
					bind:value={address}
					placeholder="wynd.example.org"
					aria-label="Адрес сервера"
					type="text"
					spellcheck={false}
					autocomplete="off"
					oninput={() => (error = '')}
					onkeydown={(e: KeyboardEvent) => {
						if (e.key === 'Enter') void submit();
					}}
				/>
				{#if error}
					<Hint>{error}</Hint>
				{/if}
				<Button
					disabled={!address.trim()}
					loading={checking}
					loadingLabel="Проверяем…"
					onclick={() => void submit()}
				>
					Продолжить
				</Button>
				<Hint centered style="margin-top:26px"
					><TextButton onclick={closeAdd}>Назад</TextButton></Hint
				>
			</div>
		</div>
	{:else if unavailable && server}
		<!-- ПК3: сервер уже выбран, сохранённого интерфейса нет. -->
		<div class="desk-content">
			<div class="welcome">
				<div class="welcome-logo"><Logo /></div>
				<h1>Сервер не отвечает</h1>
				<Hint centered>{server.name} · {server.origin.replace(/^https?:\/\//, '')}</Hint>
				<Hint centered style="margin-top:14px"
					>Проверьте сеть. Если сеть есть — сервер выключен или переехал.</Hint
				>
				<Button
					style="margin-top:22px"
					loading={checking}
					loadingLabel="Проверяем…"
					onclick={() => void retry()}
				>
					Повторить
				</Button>
				<Hint centered style="margin-top:26px"
					><TextButton onclick={() => void another()}>Другой сервер</TextButton></Hint
				>
			</div>
		</div>
	{:else if server}
		<!-- Интерфейс выбранного сервера. Он живёт в iframe, чтобы полоса
		     заголовка и рамка окна оставались нашими; запросы идут через
		     Go-прокси (internal/desktop.Proxy). -->
		{#key server.origin}
			<div class="server-host">
				<iframe src="/?wynd=server" title={server.name}></iframe>
			</div>
		{/key}
	{:else}
		<div class="desk-content">
			<div class="welcome">
				<div class="welcome-mark"><Mark /></div>
				<h1>Войти в Wynd</h1>
				<Label>Адрес сервера или приглашение</Label>
				<Input
					bind:value={address}
					placeholder="wynd.example.org"
					aria-label="Адрес сервера или приглашение"
					type="text"
					spellcheck={false}
					autocomplete="off"
					oninput={() => (error = '')}
					onkeydown={(e: KeyboardEvent) => {
						if (e.key === 'Enter') void submit();
					}}
				/>
				{#if error}
					<Hint>{error}</Hint>
				{/if}
				<Button
					disabled={!address.trim()}
					loading={checking}
					loadingLabel="Проверяем…"
					onclick={() => void submit()}
				>
					Дальше
				</Button>
			</div>
		</div>
	{/if}
</div>
