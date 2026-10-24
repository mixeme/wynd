<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import NumberField from '$ui/forms/NumberField.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatBytes } from '$lib/format/bytes';
	import { clearMediaStore, getAppSettings, mediaStoreBytes, snapshotStoreBytes, type Theme } from '$lib/idb/db';
	import { getMediaCacheLimit, setMediaCacheLimit } from '$lib/media/objectUrl';
	import { getAvatarShape, loadSessions, setAvatarShape, setTheme } from '$lib/session/session.svelte';
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

<FormLayout shell app title="Приложение" onback={() => goUp('/settings')}>
	<SectionLabel>Уведомления по умолчанию</SectionLabel>
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
	<Hint class="mt-10">Для новых кругов и тех, где настройки уведомлений не менялись.</Hint>

	<SectionLabel class="mt-22">Место на устройстве</SectionLabel>
	<Meter value={cacheBytes} max={Math.max(cacheLimit, 1)} />
	<Hint class="mt-8">
		{formatBytes(cacheBytes)} медиа и файлов из {formatBytes(cacheLimit)}.{#if textBytes > 0}{' '}Тексты для чтения без сети — {formatBytes(textBytes)}.{/if}{#if usageBytes > cacheBytes}{' '}Всего Wynd на устройстве — {formatBytes(usageBytes)}.{/if}
	</Hint>
	<ChipGroup class="mt-8">
		{#each CACHE_PRESETS as gb (gb)}
			<Chip selected={!customCache && cacheLimit === gb * GB} onclick={() => void pickCacheLimit(gb)}
				>{gb} ГБ</Chip
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
	<Hint>Сверх потолка удаляются давно не открытые медиа и файлы — при просмотре они скачаются заново.</Hint>
	<SettingsRow class="mt-6"
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

	{#if error}
		<Hint>{error}</Hint>
	{/if}
</FormLayout>
