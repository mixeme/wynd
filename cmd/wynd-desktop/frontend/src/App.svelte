<script lang="ts">
	import { onMount } from 'svelte';
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import { decodeQrFromFile, loginLinkTarget } from '$lib/auth/links';
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
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import UpdateBar from '$ui/chrome/UpdateBar.svelte';
	import {
		addInterface,
		applySelfUpdate,
		checkCurrent,
		checkSelfUpdate,
		checkServer,
		closeWindow,
		currentServer,
		currentVersion,
		forgetSelected,
		listServers,
		chooseBackupFolder,
		getBackupConfig,
		notifyWindow,
		onInviteLink,
		openBackupFolder,
		saveBackupConfig,
		openingReleases,
		previewServer,
		removeInterface,
		runtime,
		selectEmbedded,
		selectServer,
		selfUpdateProgress,
		takePendingInvite,
		interfaceSource,
		type CheckResult,
		type InterfaceSource,
		type InviteLink,
		type Update
	} from './api';

	const rt = runtime();
	let version = $state('');
	let update = $state<Update | null>(null);
	let updateHidden = $state(false);
	let updateBusy = $state(false);
	let updateBad = $state(false);
	let updateError = $state('');
	// Ход скачивания обновления, пока идёт установка.
	let updateProgress = $state({ done: 0, total: 0 });
	let progressTimer: ReturnType<typeof setInterval> | undefined;
	let maximised = $state(false);
	let address = $state('');
	let error = $state('');
	let checking = $state(false);
	// Фото с QR-кодом подключения (план 72): разбор и вход с подставленной почтой.
	let qrPicker = $state<FilePicker>();
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
	// Источник интерфейса: вшитый клиент из окна (по умолчанию) или страницы
	// выбранного сервера. Выбранный сервер остаётся источником данных.
	let source = $state<InterfaceSource>('embedded');
	// Мок-стенд кадров задаёт адрес рамки сам; в настоящем окне его нет.
	const forcedIframePath = (globalThis as unknown as { __wyndIframePath?: string }).__wyndIframePath;
	let iframePath = $state(forcedIframePath ?? '/?wynd=embedded');

	// Копии по расписанию (план 66, функция 7): экран оболочки. Настройки и
	// сборку ведёт Go, пока приложение запущено; здесь — форма.
	let backups = $state((globalThis as unknown as { __wyndBackups?: boolean }).__wyndBackups === true);
	let backupsOn = $state(false);
	let backupFolder = $state('');
	let backupSchedule = $state<'daily' | 'weekly' | 'monthly' | 'custom'>('daily');
	let backupDays = $state('3');
	let keepMode = $state<'count' | 'days'>('count');
	let backupKeep = $state('10');
	let backupError = $state('');
	let backupBusy = $state(false);
	let backupSaved = $state(false);
	let lastBackup = $state('');
	const keepHint = $derived(
		keepMode === 'count'
			? `Старые копии в этой папке удаляются сами: по каждому кругу остаются последние ${backupKeep || '0'}.`
			: `Старые копии в этой папке удаляются сами: по каждому кругу остаются копии не старше ${backupKeep || '0'} дней.`
	);

	function plural(n: number, forms: [string, string, string]): string {
		const mod10 = n % 10;
		const mod100 = n % 100;
		if (mod10 === 1 && mod100 !== 11) return forms[0];
		if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return forms[1];
		return forms[2];
	}

	function formatLast(cfg: { last_at?: string; last_circles?: number }): string {
		if (!cfg.last_at) return 'копий ещё не было';
		const at = new Date(cfg.last_at);
		if (Number.isNaN(at.getTime())) return 'копий ещё не было';
		const now = new Date();
		const sameDay =
			at.getFullYear() === now.getFullYear() &&
			at.getMonth() === now.getMonth() &&
			at.getDate() === now.getDate();
		const day = sameDay
			? 'сегодня'
			: at.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
		const time = at.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
		const count = cfg.last_circles ?? 0;
		return `${day}, ${time} — ${count} ${plural(count, ['копия', 'копии', 'копий'])}`;
	}

	async function loadBackups() {
		backupError = '';
		const cfg = await getBackupConfig();
		backupsOn = cfg.enabled;
		backupFolder = cfg.folder;
		backupSchedule = cfg.schedule;
		backupDays = String(cfg.custom_days || 3);
		keepMode = cfg.keep_mode;
		backupKeep = String(cfg.keep);
		lastBackup = formatLast(cfg);
	}

	async function pickBackupFolder() {
		try {
			const path = await chooseBackupFolder();
			if (path) backupFolder = path;
		} catch {
			backupError = 'Не получилось открыть выбор папки.';
		}
	}

	async function revealBackupFolder() {
		try {
			await openBackupFolder();
		} catch {
			backupError = 'Папка ещё не выбрана.';
		}
	}

	async function saveBackups() {
		if (backupBusy) return;
		backupBusy = true;
		backupError = '';
		backupSaved = false;
		try {
			const cfg = await saveBackupConfig({
				enabled: backupsOn,
				folder: backupFolder,
				schedule: backupSchedule,
				custom_days: Number(backupDays) || 0,
				keep_mode: keepMode,
				keep: Number(backupKeep) || 0
			});
			lastBackup = formatLast(cfg);
			backupSaved = true;
		} catch (err) {
			backupError = err instanceof Error ? err.message : 'Не получилось сохранить настройки.';
		} finally {
			backupBusy = false;
		}
	}

	onMount(async () => {
		try {
			// Источник узнаём до первой рамки, даже если сервер ещё не выбран:
			// --server-ui задаёт его на запуск. Серверный режим — явный выбор,
			// вшитый — обычный.
			source = await interfaceSource().catch(() => 'embedded');
			const saved = await currentServer();
			if (saved) {
				if (!forcedIframePath) {
					iframePath = source === 'server' ? '/?wynd=server' : '/?wynd=embedded';
				}
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
		// Приглашение из схемы wynd: (план 66, функция 4): живое событие и
		// отложенный адрес, ушедший до загрузки оболочки.
		onInviteLink(openInvite);
		const pending = await takePendingInvite().catch(() => null);
		if (pending) await openInvite(pending);
		if (backups) await loadBackups();
		void syncMaximised();
	});

	onMount(() => {
		void currentVersion()
			.then((v) => (version = v))
			.catch(() => undefined);
		void checkSelfUpdate()
			.then((u) => (update = u?.available ? u : null))
			.catch(() => undefined);
	});

	onMount(() => {
		function openFromClient(event: MessageEvent) {
			if (event.origin !== window.location.origin) return;
			if (event.data?.type === 'wynd:open-interface') {
				void openInterface();
				return;
			}
			// Первый запуск: с экрана входа (почта) назад к вводу адреса сервера
			// (план 50, находка владельца 20.11). Адрес остаётся в поле — его
			// можно поправить.
			if (event.data?.type === 'wynd:return-to-address') {
				returnToAddress(typeof event.data.origin === 'string' ? event.data.origin : '');
			}
			// Копии по расписанию (план 66, функция 7): строка «Настройки →
			// Приложение» в клиенте открывает экран оболочки.
			if (event.data?.type === 'wynd:open-backups') {
				backups = true;
				void loadBackups();
				return;
			}
			// Живой сигнал уведомления от клиента (план 66): мост гасит его,
			// когда интерфейс на виду. Заголовок обязателен — без него toast
			// теряет смысл.
			if (event.data?.type === 'wynd:notify') {
				const title = typeof event.data.title === 'string' ? event.data.title : '';
				const body = typeof event.data.body === 'string' ? event.data.body : '';
				if (title) void notifyWindow(title, body).catch(() => undefined);
				return;
			}
		}
		window.addEventListener('message', openFromClient);
		return () => window.removeEventListener('message', openFromClient);
	});

	function returnToAddress(current: string) {
		server = null;
		unavailable = false;
		showServers = false;
		adding = false;
		preview = null;
		error = '';
		checking = false;
		address = current.replace(/^https?:\/\//, '');
	}

	async function syncMaximised() {
		maximised = (await rt?.WindowIsMaximised().catch(() => false)) ?? false;
	}

	// Пока идёт установка, спрашиваем у оболочки, сколько уже скачано: окно не
	// молчит на многомегабайтном файле (C153: «Обновляем…» без конца).
	function watchProgress(on: boolean) {
		if (progressTimer) {
			clearInterval(progressTimer);
			progressTimer = undefined;
		}
		if (!on) return;
		updateProgress = { done: 0, total: 0 };
		progressTimer = setInterval(() => {
			void selfUpdateProgress()
				.then((p) => (updateProgress = p))
				.catch(() => undefined);
		}, 300);
	}

	async function applyUpdate() {
		if (updateBusy) return;
		updateBusy = true;
		updateBad = false;
		updateError = '';
		watchProgress(true);
		try {
			const res = await applySelfUpdate();
			if (res === 'signature') updateBad = true;
			else if (res) updateError = res;
		} finally {
			watchProgress(false);
			updateBusy = false;
		}
	}

	async function submit() {
		if (!address.trim() || checking) return;
		checking = true;
		error = '';
		// Ссылку подключения из QR можно вставить прямо в поле (план 72):
		// то же, что фото кода — адрес и почта, адрес вводить не надо.
		const pasted = loginLinkTarget(address);
		try {
			server = await checkServer(pasted ? pasted.origin : address.trim());
			unavailable = false;
			adding = false;
			showServers = false;
			source = await interfaceSource().catch(() => source);
			iframePath = interfacePath(pasted ? `email=${encodeURIComponent(pasted.email)}` : '');
		} catch {
			// Причина не различается: опечатка, нет сети, выключенный сервер и
			// чужой сайт выглядят одинаково (ПК4).
			error = 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.';
		} finally {
			checking = false;
		}
	}

	// Адрес корня интерфейса с явной меткой источника: рамка не должна
	// угадывать источник по заголовку запроса.
	function interfacePath(query = ''): string {
		const marker = source === 'server' ? 'wynd=server' : 'wynd=embedded';
		return query ? `/?${query}&${marker}` : `/?${marker}`;
	}

	async function readQrPhoto(file: File) {
		error = '';
		let value = '';
		try {
			value = await decodeQrFromFile(file);
		} catch {
			error = 'Не удалось прочитать код — попробуйте другое фото';
			return;
		}
		// Ссылка подключения (план 72): адрес и почта одним кодом.
		const link = loginLinkTarget(value);
		checking = true;
		try {
			server = await checkServer(link ? link.origin : value.trim());
			unavailable = false;
			adding = false;
			showServers = false;
			source = await interfaceSource().catch(() => source);
			iframePath = interfacePath(link ? `email=${encodeURIComponent(link.email)}` : '');
		} catch {
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
		source = 'embedded';
		unavailable = false;
		address = '';
		error = '';
	}

	async function openInterface() {
		servers = await listServers().catch(() => []);
		source = await interfaceSource().catch(() => 'embedded');
		error = '';
		adding = false;
		preview = null;
		showServers = true;
	}

	function closeInterface() {
		error = '';
		showServers = false;
		iframePath = '/settings/app';
	}

	async function choose(origin: string) {
		try {
			const next = await selectServer(origin);
			server = next;
			source = 'server';
			unavailable = false;
			iframePath = '/?wynd=server';
			showServers = false;
		} catch {
			error = 'Не удалось открыть сервер. Попробуйте ещё раз.';
		}
	}

	// Возврат к вшитому интерфейсу: выбранный сервер остаётся источником
	// данных, меняются только страницы.
	async function chooseEmbedded() {
		if (source === 'embedded') {
			showServers = false;
			return;
		}
		try {
			await selectEmbedded();
			source = 'embedded';
			error = '';
			iframePath = '/?wynd=embedded';
			showServers = false;
		} catch {
			error = 'Не удалось вернуться к интерфейсу приложения. Попробуйте ещё раз.';
		}
	}

	// Приглашение из браузера (план 66, функция 4): человек нажал
	// «Открыть в приложении» на странице приглашения. Свой сервер — прямо
	// на экран приглашения; чужой — интерфейс переключается на приглашённый
	// сервер (проверка адреса та же, что при запуске); отказ проверки —
	// экран адреса с этим адресом и словами об отказе: поправить связь или
	// повторить проверку можно тут же.
	async function openInvite(link: InviteLink) {
		try {
			if (server && server.origin === link.origin) {
				unavailable = false;
				iframePath = link.path;
				return;
			}
			server = await checkServer(link.origin);
			unavailable = false;
			showServers = false;
			iframePath = link.path;
		} catch {
			address = link.origin;
			preview = null;
			checking = false;
			error = 'Сервер приглашения не отвечает. Проверьте связь и попробуйте ещё раз.';
			showServers = false;
			adding = true;
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
		title={`Wynd Настольный${version ? ` [${version}]` : ''}`}
		{maximised}
		onminimise={() => rt?.WindowMinimise()}
		ontoggle={() => {
			rt?.WindowToggleMaximise();
			void syncMaximised();
		}}
		onclose={() => void closeWindow()}
	/>
	{#if update && !updateHidden}
		<!-- Полоса обновления поверх интерфейса: страница остаётся во всю
		     высоту, полоса занимает верхнюю часть, пока её не скроют. -->
		<div class="desk-update desk-update-bar">
			<UpdateBar
				class="desk-update-inner"
				head={`Вышла версия ${update.version}`}
				bad={updateBad}
				error={updateError}
				busy={updateBusy}
				done={updateProgress.done}
				total={updateProgress.total}
				attestation={!!update.attestation}
				onapply={() => void applyUpdate()}
				onhide={() => (updateHidden = true)}
				onreleases={() => void openingReleases()}
			/>
		</div>
	{/if}
	{#if !loaded}
		<!-- Первое чтение списка серверов: окно ещё не решает, что показать. -->
	{:else if backups}
		<!-- Кадр: копии по расписанию (план 66, функция 7). -->
		<div class="page-host">
			<div class="settings-page">
				<BackBar title="Копии по расписанию" onback={() => (backups = false)} />
				<div class="settings-body">
					<SettingsRow title="Сохранять копии по расписанию">
						{#snippet control()}
							<Switch bind:checked={backupsOn} label="Сохранять копии по расписанию" />
						{/snippet}
					</SettingsRow>
					<Hint>
						Каждый круг сохраняется отдельным ZIP-архивом в выбранную папку — как «Скачать
						архив», только сам.
					</Hint>

					{#if backupsOn}
						<Label>Папка</Label>
						<div class="backup-row">
							<FieldDisplay mono value={backupFolder} />
							<Button variant="ghost" onclick={() => void revealBackupFolder()}>Открыть</Button>
							<Button variant="ghost" onclick={() => void pickBackupFolder()}>Выбрать…</Button>
						</div>

						<Label>Как часто</Label>
						<ChipGroup>
							<Chip selected={backupSchedule === 'daily'} onclick={() => (backupSchedule = 'daily')}>
								Каждый день
							</Chip>
							<Chip selected={backupSchedule === 'weekly'} onclick={() => (backupSchedule = 'weekly')}>
								Каждую неделю
							</Chip>
							<Chip selected={backupSchedule === 'monthly'} onclick={() => (backupSchedule = 'monthly')}>
								Каждый месяц
							</Chip>
							<Chip selected={backupSchedule === 'custom'} onclick={() => (backupSchedule = 'custom')}>
								Свой интервал
							</Chip>
						</ChipGroup>
						{#if backupSchedule === 'custom'}
							<Label>Свой интервал</Label>
							<div class="backup-row">
								<Hint>каждые</Hint>
								<Input small bind:value={backupDays} aria-label="Интервал, дней" />
								<Hint>дней</Hint>
							</div>
						{/if}
						<Hint>Копия собирается при запущенном приложении — окно можно закрыть.</Hint>

						<Label>Хранить копии</Label>
						<div class="backup-row">
							<Input
								small
								style="max-width:96px"
								bind:value={backupKeep}
								aria-label="Сколько копий хранить"
							/>
							<ChipGroup>
								<Chip selected={keepMode === 'count'} onclick={() => (keepMode = 'count')}>
									последних
								</Chip>
								<Chip selected={keepMode === 'days'} onclick={() => (keepMode = 'days')}>
									дней
								</Chip>
							</ChipGroup>
						</div>
						<Hint>{keepHint}</Hint>
					{/if}

					<Label>Последняя копия</Label>
					<Hint>{lastBackup}</Hint>

					{#if backupError}
						<Hint class="mt-8">{backupError}</Hint>
					{/if}
					{#if backupSaved}
						<Hint class="mt-8">Настройки сохранены.</Hint>
					{/if}
					<Button loading={backupBusy} loadingLabel="Сохраняем…" onclick={() => void saveBackups()}>Сохранить</Button>
				</div>
			</div>
		</div>
	{:else if showServers}
		<!-- Кадр 5: источники интерфейса — вшитый клиент и серверы; учётные
		     записи живут отдельно. -->
		<div class="page-host">
			<div class="settings-page">
				<BackBar title="Источник интерфейса" onback={closeInterface} />
				<div class="settings-body">
					<Hint class="mt-16">Интерфейс открывается из приложения или с выбранного сервера. Входы и круги останутся без изменений.</Hint>
					{#if source === 'embedded'}
						<ServerRow
							name="Встроенный"
							subtitle="Интерфейс приложения"
							detail="Сейчас открывает интерфейс"
							variant="ok"
						/>
					{:else}
						<ServerRow
							name="Встроенный"
							subtitle="Интерфейс приложения"
							onclick={() => void chooseEmbedded()}
						/>
					{/if}
					<Label>Серверы интерфейса</Label>
					{#each servers as item (item.origin)}
						{#if source === 'server' && item.origin === server?.origin}
							<ServerRow
								name={item.name}
								subtitle={item.origin.replace(/^https?:\/\//, '')}
								variant="ok"
								onclick={() => void choose(item.origin)}
							/>
						{:else if item.origin === server?.origin}
							<!-- С этого сервера идут данные, но страницы — вшитые:
							     убрать его из списка, пока он выбран, нельзя. -->
							<ServerRow
								name={item.name}
								subtitle={item.origin.replace(/^https?:\/\//, '')}
								detail="Данные с этого сервера"
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
				<ServerRow
					name={server.name}
					subtitle={server.origin.replace(/^https?:\/\//, '')}
					connection={checking ? 'checking' : 'unavailable'}
					card
				/>
				<Hint centered style="margin-top:14px"
					>Проверьте сеть. Если сеть есть — сервер выключен или переехал.</Hint
				>
				<Button
					style="margin-top:22px"
					stableHeight
					loading={checking}
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
				<Button variant="ghost" onclick={() => qrPicker?.open()}>Фото с QR-кодом</Button>
				<Hint>Ссылку из QR-кода можно вставить прямо в поле.</Hint>
			</div>
		</div>
	{/if}
	<FilePicker bind:this={qrPicker} accept="image/*" onfiles={([file]) => void readQrPhoto(file)} />
	{#if removeTarget}
		<ConfirmDialog
			title={`Убрать сервер «${removeTarget.name}» из списка?`}
			confirmLabel="Убрать"
			cancelLabel="Оставить"
			loading={removing}
			onconfirm={() => void confirmRemove()}
			oncancel={() => (removeTarget = null)}
		>
			<Hint class="mb-16">Сервер исчезнет только из источников интерфейса. Вход и данные останутся. Добавить его снова можно по адресу.</Hint>
		</ConfirmDialog>
	{/if}
</div>
