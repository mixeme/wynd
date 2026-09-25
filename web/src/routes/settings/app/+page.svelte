<script lang="ts">
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
	import { appVersion } from '$lib/appinfo';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatBytes } from '$lib/format/bytes';
	import { clearMediaStore, getAppSettings, mediaStoreBytes, type Theme } from '$lib/idb/db';
	import { loadSessions, setTheme } from '$lib/session/session.svelte';
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
	let freeBytes = $state(0);
	let ready = $state(false);
	let baseline = $state<AppNotifyDefaults | null>(null);
	let error = $state('');
	let cleared = $state(false);

	const themes: { key: Theme; label: string }[] = [
		{ key: 'system', label: 'Как в системе' },
		{ key: 'light', label: 'Светлая' },
		{ key: 'dark', label: 'Тёмная' }
	];

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
		cacheBytes = await mediaStoreBytes();
		if (navigator.storage?.estimate) {
			const est = await navigator.storage.estimate();
			freeBytes = Math.max(0, (est.quota ?? 0) - (est.usage ?? 0));
		}
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

<FormLayout shell app title="Приложение" onback={() => goto('/settings')}>
	<SectionLabel>Уведомления по умолчанию</SectionLabel>
	<SettingsRow title="Новые записи" style="padding-top:2px">
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
	<Hint style="margin-top:10px">
		Эти переключатели действуют в кругах, для которых вы не задавали отдельные уведомления, и в
		тех, в которые вы вступите позже. Круги, где уведомления уже сохранены отдельно, не меняются.
	</Hint>

	<SectionLabel style="margin-top:22px">Место на устройстве</SectionLabel>
	<Meter value={cacheBytes} max={Math.max(cacheBytes + freeBytes, 1)} />
	<Hint style="margin-top:8px">
		{formatBytes(cacheBytes)} кэша{#if freeBytes}
			{' '}· {formatBytes(freeBytes)} свободно{/if}
	</Hint>
	<SettingsRow
		title="Очистить кэш"
		subtitle="фотографии скачаются заново при просмотре"
		chevron={false}
		style="margin-top:6px"
		onclick={() => void clearCache()}
	/>
	{#if cleared}
		<Hint>Кэш очищен. Оригиналы на сервере на месте.</Hint>
	{/if}

	<SectionLabel style="margin-top:20px">Тема</SectionLabel>
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

	<Hint style="margin-top:20px">Wynd {appVersion} · AGPL-3.0</Hint>
	{#if error}
		<Hint>{error}</Hint>
	{/if}
</FormLayout>
