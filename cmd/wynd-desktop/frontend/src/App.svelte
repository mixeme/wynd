<script lang="ts">
	import { onMount } from 'svelte';
	import TitleBar from '$ui/workshop/TitleBar.svelte';
	import Mark from '$ui/Mark.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Logo from '$ui/Logo.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import {
		checkCurrent,
		checkServer,
		currentServer,
		forgetSelected,
		runtime,
		type CheckResult
	} from './api';

	const rt = runtime();
	let maximised = $state(false);
	let address = $state('');
	let error = $state('');
	let checking = $state(false);
	let server = $state<CheckResult | null>(null);
	let unavailable = $state(false);
	let loaded = $state(false);

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
	/>
	{#if !loaded}
		<!-- Первое чтение списка серверов: окно ещё не решает, что показать. -->
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
		<div class="server-host">
			<iframe src="/?wynd=server" title={server.name}></iframe>
		</div>
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
