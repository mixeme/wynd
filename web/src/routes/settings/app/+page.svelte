<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { androidInterfacesAvailable } from '$lib/android/interfaces';
	import { androidPushAvailable, androidPushStatus } from '$lib/android/push';
	import { setAndroidPushEnabled } from '$lib/push/push';
	import NumberField from '$ui/forms/NumberField.svelte';
	import { onMount } from 'svelte';
	import { androidUpdate, setAndroidImmediate } from '$lib/android/update.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatBytes } from '$lib/format/bytes';
	import {
		clearMediaStore,
		getAppSettings,
		mediaStoreBytes,
		snapshotStoreBytes,
		type Theme
	} from '$lib/idb/db';
	import { getMediaCacheLimit, setMediaCacheLimit } from '$lib/media/objectUrl';
	import { autostartBridge, autostartSystemName } from '$lib/desktop/autostart';
	import { openDesktopBackups } from '$lib/desktop/servers';
	import { desktopBridge } from '$lib/desktop/bridge';
	import { openDesktopInterface } from '$lib/desktop/servers';
	import { displayHost } from '$lib/auth/origin';
	import { loadServerCard } from '$lib/instance/serverCard';
	import {
		getAvatarShape,
		loadSessions,
		setAvatarShape,
		setTheme
	} from '$lib/session/session.svelte';
	import {
		accountNotifyPrefsFromAppDefaults,
		appNotifyDefaultsEqual,
		fetchAccountNotifyPrefs,
		persistNotifyDefaults,
		saveAccountNotifyPrefs,
		type AppNotifyDefaults
	} from '$lib/settings/notify';

	let posts = $state(true);
	let commentsMine = $state(true);
	let reactions = $state(false);
	let theme = $state<Theme>('system');
	let cacheBytes = $state(0);
	// Тексты для чтения без сети — снимки экранов (C46): отдельно, чтобы видеть рост.
	let textBytes = $state(0);
	// Сколько браузер отводит сайту (storage.estimate().quota). Это не свободное
	// место на диске — его браузер странице не сообщает; Firefox, например,
	// даёт около 10 ГБ при любом телефоне. Раньше здесь было «свободно на
	// устройстве», и цифра вводила в заблуждение.
	let quotaBytes = $state(0);
	// Всё, что Wynd держит на устройстве (storage.estimate().usage): кроме фото
	// и видео — снимки журнала, очередь отправки, само приложение (план 46, C16).
	let usageBytes = $state(0);
	// Потолок кэша: 1 / 2 / 5 ГБ или своё число гигабайт (1–100).
	const GB = 1024 * 1024 * 1024;
	const CACHE_PRESETS = [1, 2, 5] as const;
	let cacheLimit = $state(2 * GB);
	let customCacheGb = $state(10);
	let customCache = $state(false);
	let ready = $state(false);
	let baseline = $state<AppNotifyDefaults | null>(null);
	let error = $state('');
	let cleared = $state(false);
	// Тот же принятый раздел интерфейса в обеих нативных оболочках (планы 62 и 50).
	const desktop = desktopBridge();
	const androidInterfaces = androidInterfacesAvailable();
	const interfaceOrigin = desktop?.origin ?? (androidInterfaces ? window.location.origin : '');
	let interfaceName = $state(interfaceOrigin ? displayHost(interfaceOrigin) : '');
	// Автозапуск — только в окне для компьютера; в браузере моста нет.
	const auto = autostartBridge();
	const autostartSystem = autostartSystemName();
	let autostart = $state(false);
	let autostartBusy = $state(false);
	let autostartStale = $state(false);
	let autostartError = $state('');
	const nativePush = androidPushAvailable();
	let pushEnabled = $state(false);
	let pushBusy = $state(true);
	let pushError = $state('');
	let pushDistributorMissing = $state(false);

	const themes: { key: Theme; label: string }[] = [
		{ key: 'system', label: 'Как в системе' },
		{ key: 'light', label: 'Светлая' },
		{ key: 'dark', label: 'Тёмная' }
	];
	const avatarShapes = [
		{ key: 'circle', label: 'Круг' },
		{ key: 'square', label: 'Скруглённый квадрат' }
	] as const;

	function currentDefaults(): AppNotifyDefaults {
		return { posts, comments_mine: commentsMine, reactions };
	}

	async function togglePush(enabled: boolean) {
		if (pushBusy) return;
		pushBusy = true;
		pushError = '';
		pushDistributorMissing = false;
		try {
			const status = await setAndroidPushEnabled(enabled);
			pushEnabled = status.enabled && status.allowed;
			if (status.reason === 'permission') {
				pushError = 'Разрешите уведомления для Wynd в настройках телефона.';
			} else if (status.reason === 'distributor') {
				pushDistributorMissing = true;
			}
		} catch {
			pushEnabled = !enabled;
			pushError = 'Не удалось изменить уведомления. Попробуйте ещё раз.';
		} finally {
			pushBusy = false;
		}
	}

	async function toggleAutostart(next: boolean) {
		if (!auto || autostartBusy) return;
		const before = !next;
		autostartBusy = true;
		autostartError = '';
		try {
			const state = await auto.set(next);
			autostart = state.enabled;
			autostartStale = state.stale;
		} catch {
			autostart = before;
			autostartError = 'Не удалось изменить автозапуск. Попробуйте ещё раз.';
		} finally {
			autostartBusy = false;
		}
	}

	async function persistPrefs() {
		if (!ready || !baseline) return;
		const next = currentDefaults();
		if (appNotifyDefaultsEqual(next, baseline)) return;
		const accountPrefs = accountNotifyPrefsFromAppDefaults(next);
		try {
			await persistNotifyDefaults(next);
			const sessions = await loadSessions();
			for (const session of sessions) {
				await saveAccountNotifyPrefs(session.origin, accountPrefs);
			}
			baseline = next;
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	$effect(() => {
		void posts;
		void commentsMine;
		void reactions;
		void persistPrefs();
	});

	async function refreshCache() {
		cacheLimit = await getMediaCacheLimit();
		const gb = Math.round(cacheLimit / GB);
		customCache = !(CACHE_PRESETS as readonly number[]).includes(gb);
		if (customCache) customCacheGb = gb;
		cacheBytes = await mediaStoreBytes();
		textBytes = await snapshotStoreBytes().catch(() => 0);
		if (navigator.storage?.estimate) {
			const est = await navigator.storage.estimate();
			quotaBytes = est.quota ?? 0;
			usageBytes = est.usage ?? 0;
		}
	}

	async function pickCacheLimit(gb: number, custom = false) {
		customCache = custom;
		const clamped = Math.max(1, Math.min(100, Math.round(Number(gb) || 1)));
		if (custom) customCacheGb = clamped;
		await setMediaCacheLimit(clamped * GB);
		await refreshCache();
	}

	async function clearCache() {
		await clearMediaStore();
		cleared = true;
		await refreshCache();
	}

	onMount(async () => {
		if (nativePush) {
			void androidPushStatus()
				.then((status) => (pushEnabled = status.enabled && status.allowed))
				.catch(() => (pushError = 'Не удалось прочитать настройку уведомлений.'))
				.finally(() => (pushBusy = false));
		}
		if (interfaceOrigin) {
			const card = await loadServerCard(interfaceOrigin);
			interfaceName = card.info?.name || displayHost(interfaceOrigin);
		}
		if (auto) {
			try {
				const state = await auto.get();
				autostart = state.enabled;
				autostartStale = state.stale;
			} catch {
				autostartError = 'Не удалось прочитать автозапуск. Попробуйте ещё раз.';
			}
		}
		const settings = await getAppSettings();
		theme = settings?.theme ?? 'system';
		const defaults = settings?.notify_defaults;
		if (defaults) {
			posts = defaults.posts ?? true;
			commentsMine = defaults.comments_mine ?? defaults.comments ?? true;
			reactions = defaults.reactions ?? false;
		}
		const sessions = await loadSessions();
		if (sessions[0]) {
			try {
				const prefs = await fetchAccountNotifyPrefs(sessions[0].origin);
				posts = prefs.posts;
				commentsMine = prefs.comments_mine;
				reactions = prefs.reactions;
			} catch {
				/* local defaults */
			}
		}
		await refreshCache();
		baseline = currentDefaults();
		ready = true;
	});
</script>

<FormLayout shell app title="Приложение" onback={navigateBack}>
	{#if interfaceOrigin}
		<SectionLabel>Интерфейс</SectionLabel>
		<ServerRow
			name={interfaceName}
			subtitle={displayHost(interfaceOrigin)}
			detail="Сейчас открывает интерфейс"
			card
			top={0}
		/>
		<SettingsRow
			title="Сменить сервер интерфейса"
			subtitle="Входы и круги останутся на своих серверах"
			onclick={() => {
				if (desktop) openDesktopInterface();
				else void goto('/settings/app/interface');
			}}
		/>
	{/if}
	{#if auto}
		<SectionLabel>Запуск</SectionLabel>
		<Hint class="mt-8 mb-8">При входе в {autostartSystem} откроется Wynd.</Hint>
		<SettingsRow class="pt-2" title="Запускать при входе в {autostartSystem}">
			{#snippet control()}
				<Switch
					bind:checked={autostart}
					disabled={autostartBusy}
					label="Запускать при входе в {autostartSystem}"
					onchange={(v) => void toggleAutostart(v)}
				/>
			{/snippet}
		</SettingsRow>
		{#if autostartStale}
			<Hint class="mt-8"
				>Автозапуск указывает на прежний файл. Включите его заново, чтобы обновить команду.</Hint
			>
		{/if}
		{#if autostartError}
			<Hint class="mt-8">{autostartError}</Hint>
		{/if}
	{/if}

	{#if desktop}
		<SectionLabel>Копии</SectionLabel>
		<SettingsRow
			class="pt-2"
			title="Копии по расписанию"
			subtitle="Архивы кругов в папку на этом компьютере"
			onclick={openDesktopBackups}
		/>
	{/if}

	{#if nativePush}
		<SettingsRow title="Уведомления на этом телефоне">
			{#snippet control()}
				<Switch
					bind:checked={pushEnabled}
					disabled={pushBusy}
					label="Уведомления на этом телефоне"
					onchange={(value) => void togglePush(value)}
				/>
			{/snippet}
		</SettingsRow>
		{#if pushDistributorMissing}
			<Hint class="mt-8">
				В этой версии приложения нет пуш-уведомлений через сервисы Google — их доставляет отдельная
				программа. Подойдёт любое приложение из
				<a class="under" href="https://unifiedpush.org/users/distributors/"
					>списка доставщиков UnifiedPush</a
				>: установите одно и включите переключатель ещё раз.
			</Hint>
		{:else if pushError}<Hint class="mt-8">{pushError}</Hint>{/if}
	{/if}
	<SectionLabel>Уведомления по умолчанию</SectionLabel>
	<Hint class="mt-8 mb-8">Для новых кругов и тех, где настройки уведомлений не менялись.</Hint>
	<SettingsRow class="pt-2" title="Новые записи">
		{#snippet control()}
			<Switch bind:checked={posts} label="Новые записи" />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Комментарии к моим записям">
		{#snippet control()}
			<Switch bind:checked={commentsMine} label="Комментарии к моим записям" />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Реакции">
		{#snippet control()}
			<Switch bind:checked={reactions} label="Реакции" />
		{/snippet}
	</SettingsRow>
	<SectionLabel class="mt-16">Место на устройстве</SectionLabel>
	<Meter value={cacheBytes} max={Math.max(cacheLimit, 1)} />
	<Hint class="mt-8">
		{formatBytes(cacheBytes)} медиа и файлов из {formatBytes(
			cacheLimit
		)}.{#if textBytes > 0}{' '}Тексты для чтения без сети — {formatBytes(
				textBytes
			)}.{/if}{#if usageBytes > cacheBytes}{' '}Всего Wynd на устройстве — {formatBytes(
				usageBytes
			)}.{/if}
	</Hint>
	<ChipGroup class="mt-8">
		{#each CACHE_PRESETS as gb (gb)}
			<Chip
				selected={!customCache && cacheLimit === gb * GB}
				onclick={() => void pickCacheLimit(gb)}>{gb} ГБ</Chip
			>
		{/each}
		<Chip selected={customCache} onclick={() => (customCache = true)}>Своё…</Chip>
	</ChipGroup>
	{#if customCache}
		<NumberField
			bind:value={customCacheGb}
			min={1}
			max={100}
			onchange={() => void pickCacheLimit(customCacheGb, true)}
			unit="ГБ, до 100"
		/>
	{/if}
	<!-- Под полем, а не между чипами и полем: строка отвечает на введённое число (C44). -->
	{#if quotaBytes && cacheLimit > quotaBytes}
		<Hint>Браузер отводит Wynd до {formatBytes(quotaBytes)} — больше кэш не вырастет.</Hint>
	{/if}
	<Hint
		>Сверх потолка удаляются давно не открытые медиа и файлы — при просмотре они скачаются заново.</Hint
	>
	<SettingsRow
		class="mt-8"
		title="Очистить кэш"
		subtitle={cleared ? 'кэш очищен' : undefined}
		chevron={false}
		onclick={() => void clearCache()}
	/>

	<!-- A16: одна форма на все устройства (владелец, 2026-10-24), над «Темой». -->
	<SectionLabel class="mt-8">Аватары</SectionLabel>
	<ChipGroup>
		{#each avatarShapes as item (item.key)}
			<Chip selected={getAvatarShape() === item.key} onclick={() => void setAvatarShape(item.key)}>
				{item.label}
			</Chip>
		{/each}
	</ChipGroup>

	<SectionLabel class="mt-16">Тема</SectionLabel>
	<ChipGroup>
		{#each themes as item (item.key)}
			<Chip
				selected={theme === item.key}
				onclick={() => {
					theme = item.key;
					setTheme(item.key);
				}}
			>
				{item.label}
			</Chip>
		{/each}
	</ChipGroup>

	{#if androidUpdate.supported && (androidUpdate.development || androidUpdate.debug)}
		<SectionLabel class="mt-16">Разработка</SectionLabel>
		<SettingsRow title="Скачивать сразу" subtitle="Не ждать 72 часа после публикации">
			{#snippet control()}
				<Switch
					checked={androidUpdate.immediately}
					disabled={androidUpdate.busy || androidUpdate.installing}
					label="Скачивать сразу"
					onchange={(value) => void setAndroidImmediate(value)}
				/>
			{/snippet}
		</SettingsRow>
		<Hint>Только для разработки. Подпись и версию проверяем всегда.</Hint>
	{/if}
	{#if error}
		<Hint>{error}</Hint>
	{/if}
</FormLayout>
