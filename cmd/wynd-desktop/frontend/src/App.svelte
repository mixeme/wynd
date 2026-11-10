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
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import {
		addInterface,
		checkCurrent,
		checkServer,
		currentServer,
		forgetSelected,
		listServers,
		previewServer,
		removeInterface,
		runtime,
		selectServer,
		type CheckResult
	} from './api';

	const rt = runtime();
	let maximised = $state(false);
	let address = $state('');
	let error = $state('');
	let checking = $state(false);
	let server = $state<CheckResult | null>(null);
	let preview = $state<CheckResult | null>(null);
	let servers = $state<CheckResult[]>([]);
	let unavailable = $state(false);
	let loaded = $state(false);
	let showServers = $state(false);
	let adding = $state(false);
	let addBusy = $state(false);
	let removeTarget = $state<CheckResult | null>(null);
	let removing = $state(false);
	let checkGeneration = 0;
	let iframePath = $state('/?wynd=server');

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

	onMount(() => {
		function openFromClient(event: MessageEvent) {
			if (event.origin !== window.location.origin || event.data?.type !== 'wynd:open-interface') {
				return;
			}
			void openInterface();
		}
		window.addEventListener('message', openFromClient);
		return () => window.removeEventListener('message', openFromClient);
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

	async function openInterface() {
		servers = await listServers().catch(() => []);
		error = '';
		adding = false;
		preview = null;
		showServers = true;
	}

	function closeInterface() {
		error = '';
		showServers = false;
		iframePath = '/settings/app?wynd=server';
	}

	async function choose(origin: string) {
		try {
			const next = await selectServer(origin);
			server = next;
			unavailable = false;
			iframePath = '/?wynd=server';
			showServers = false;
		} catch {
			error = 'Не удалось открыть сервер. Попробуйте ещё раз.';
		}
	}

	function openAdd() {
		address = '';
		preview = null;
		error = '';
		checking = false;
		checkGeneration++;
		showServers = false;
		adding = true;
	}

	function closeAdd() {
		adding = false;
		preview = null;
		error = '';
		showServers = true;
	}

	function resetPreview() {
		preview = null;
		error = '';
		checking = false;
		checkGeneration++;
	}

	async function checkAddress() {
		const value = address.trim();
		const generation = ++checkGeneration;
		preview = null;
		error = '';
		if (!value) {
			checking = false;
			return;
		}
		checking = true;
		try {
			const result = await previewServer(value);
			if (generation === checkGeneration) preview = result;
		} catch {
			if (generation === checkGeneration) {
				error = 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.';
			}
		} finally {
			if (generation === checkGeneration) checking = false;
		}
	}

	async function addCheckedInterface() {
		if (!preview || checking || addBusy) return;
		addBusy = true;
		error = '';
		try {
			await addInterface(preview.origin);
			servers = await listServers();
			preview = null;
			address = '';
			adding = false;
			showServers = true;
		} catch {
			error = 'Не удалось добавить сервер интерфейса. Попробуйте ещё раз.';
		} finally {
			addBusy = false;
		}
	}

	async function confirmRemove() {
		const target = removeTarget;
		if (!target || removing) return;
		removing = true;
		error = '';
		try {
			await removeInterface(target.origin);
			servers = await listServers();
			removeTarget = null;
		} catch {
			removeTarget = null;
			error = 'Не удалось убрать сервер из списка. Попробуйте ещё раз.';
		} finally {
			removing = false;
		}
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
	{:else if showServers}
		<!-- Кадр 5: только источники интерфейса, не учётные записи. -->
		<div class="page-host">
			<div class="settings-page">
				<BackBar title="Сервер интерфейса" onback={closeInterface} />
				<div class="settings-body">
					<Hint class="mt-16">Окно загружает интерфейс с выбранного сервера. Ваши входы и круги останутся без изменений.</Hint>
					<Label>Серверы интерфейса</Label>
					{#each servers as item (item.origin)}
						{#if item.origin === server?.origin}
							<ServerRow
								name={item.name}
								subtitle={item.origin.replace(/^https?:\/\//, '')}
								variant="ok"
								onclick={() => void choose(item.origin)}
							/>
						{:else}
							<div class="removable-server">
								<ServerRow
									name={item.name}
									subtitle={item.origin.replace(/^https?:\/\//, '')}
									style="padding-right:52px"
									onclick={() => void choose(item.origin)}
								/>
								<IconButton
									name="x"
									size="sm"
									label={`Убрать сервер «${item.name}» из списка`}
									onclick={() => (removeTarget = item)}
								/>
							</div>
						{/if}
					{/each}
					{#if error}
						<Hint class="mt-16">{error}</Hint>
					{/if}
					<Button onclick={openAdd}>Добавить сервер интерфейса</Button>
				</div>
			</div>
		</div>
	{:else if adding}
		<!-- Кадр 6: проверка и добавление без входа и переключения. -->
		<div class="page-host">
			<div class="settings-page">
				<BackBar title="Добавить сервер интерфейса" onback={closeAdd} />
				<div class="settings-body">
					<Label>Адрес сервера</Label>
					<Input
						active
						mono
						small
						bind:value={address}
						placeholder="wynd.example.org"
						aria-label="Адрес сервера"
						type="text"
						spellcheck={false}
						autocomplete="off"
						oninput={resetPreview}
						onblur={() => void checkAddress()}
						onkeydown={(e: KeyboardEvent) => {
							if (e.key === 'Enter') {
								if (preview) void addCheckedInterface();
								else (e.currentTarget as HTMLInputElement).blur();
							}
						}}
					/>
					{#if preview}
						<ServerRow
							name={preview.name}
							subtitle={preview.origin.replace(/^https?:\/\//, '')}
							detail="Сервер доступен"
							card
						/>
					{:else if checking}
						<Hint class="mt-8">Проверяем сервер…</Hint>
					{/if}
					<Hint>Сервер появится в списке источников интерфейса. Вход и учётная запись не создаются.</Hint>
					{#if error}
						<Hint>{error}</Hint>
					{/if}
					<Button
						disabled={!preview || checking}
						loading={addBusy}
						loadingLabel="Добавляем…"
						onclick={() => void addCheckedInterface()}
					>
						Добавить сервер интерфейса
					</Button>
				</div>
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
				<iframe src={iframePath} title={server.name}></iframe>
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
	{#if removeTarget}
		<ConfirmDialog
			title={`Убрать сервер «${removeTarget.name}» из списка?`}
			confirmLabel="Убрать"
			cancelLabel="Оставить"
			loading={removing}
			onconfirm={() => void confirmRemove()}
			oncancel={() => (removeTarget = null)}
		>
			<Hint class="mb-14">Сервер исчезнет только из источников интерфейса. Вход и данные останутся. Добавить его снова можно по адресу.</Hint>
		</ConfirmDialog>
	{/if}
</div>
