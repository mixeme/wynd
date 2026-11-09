<script lang="ts">
	import { onMount } from 'svelte';
	import TitleBar from '$ui/workshop/TitleBar.svelte';
	import Mark from '$ui/Mark.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import { checkServer, currentServer, runtime, type CheckResult } from './api';

	const registrationDetails: Record<string, string> = {
		open: 'Открыт для новых',
		invite: 'Только по приглашению',
		closed: 'Регистрация закрыта'
	};

	const rt = runtime();
	let maximised = $state(false);
	let address = $state('');
	let error = $state('');
	let checking = $state(false);
	let server = $state<CheckResult | null>(null);
	let loaded = $state(false);

	onMount(async () => {
		try {
			server = await currentServer();
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
		} catch {
			// Причина не различается: опечатка, нет сети, выключенный сервер и
			// чужой сайт выглядят одинаково (ПК4).
			error = 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.';
		} finally {
			checking = false;
		}
	}

	function another() {
		server = null;
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
	<div class="desk-content">
		{#if !loaded}
			<!-- Первое чтение списка серверов: окно ещё не решает, что показать. -->
		{:else if server}
			<!-- Шаг 2: проверка и сохранение адреса. Интерфейс сервера и вход —
			     следующие шаги плана 61. -->
			<div class="welcome">
				<div class="welcome-mark"><Mark /></div>
				<h1>Сервер выбран</h1>
				<ServerRow
					name={server.name}
					subtitle={server.origin.replace(/^https?:\/\//, '')}
					detail={registrationDetails[server.registration_mode] ?? ''}
					card
					top={0}
				/>
				<Hint class="mt-16">Сервер сохранён. Интерфейс и вход подключим следующим шагом.</Hint>
				<Hint centered><TextButton onclick={another}>Другой сервер</TextButton></Hint>
			</div>
		{:else}
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
		{/if}
	</div>
</div>
