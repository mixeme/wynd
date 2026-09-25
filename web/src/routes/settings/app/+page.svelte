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
	import { getTheme, loadSessions, setTheme } from '$lib/session/session.svelte';
	import {
		fetchAccountNotifyPrefs,
		persistNotifyDefaults,
		saveAccountNotifyPrefs
	} from '$lib/settings/notify';

	type NotifyPrefs = { posts: boolean; comments: boolean; reactions: boolean };

	let posts = $state(true);
	let comments = $state(true);
	let reactions = $state(false);
	let theme = $state<Theme>('system');
	let cacheBytes = $state(0);
	let freeBytes = $state(0);
	let ready = $state(false);
	let baseline = $state<NotifyPrefs | null>(null);
	let error = $state('');
	let cleared = $state(false);

	const themes: { key: Theme; label: string }[] = [
		{ key: 'system', label: 'Как в системе' },
		{ key: 'light', label: 'Светлая' },
		{ key: 'dark', label: 'Тёмная' }
	];

	async function persistPrefs() {
		if (!ready || !baseline) return;
		if (
			posts === baseline.posts &&
			comments === baseline.comments &&
			reactions === baseline.reactions
		) {
			return;
		}
		try {
			await persistNotifyDefaults({ posts, comments, reactions });
			const sessions = await loadSessions();
			for (const session of sessions) {
				await saveAccountNotifyPrefs(session.origin, { posts, comments, reactions });
			}
			baseline = { posts, comments, reactions };
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	$effect(() => {
		void posts;
		void comments;
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
		theme = getTheme();
		const settings = await getAppSettings();
		if (settings?.notify_defaults) {
			posts = settings.notify_defaults.posts ?? true;
			comments = settings.notify_defaults.comments ?? true;
			reactions = settings.notify_defaults.reactions ?? false;
		}
		const sessions = await loadSessions();
		if (sessions[0]) {
			try {
				const prefs = await fetchAccountNotifyPrefs(sessions[0].origin);
				posts = prefs.posts;
				comments = prefs.comments;
				reactions = prefs.reactions;
			} catch {
				/* local defaults */
			}
		}
		await refreshCache();
		baseline = { posts, comments, reactions };
		ready = true;
	});
</script>

<FormLayout shell app title="Приложение" onback={() => goto('/settings')}>
	<SectionLabel>Уведомления по умолчанию</SectionLabel>
	<SettingsRow title="Новые записи" style="padding-top:2px">
		{#snippet control()}
			<Switch bind:checked={posts} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Комментарии к моим записям">
		{#snippet control()}
			<Switch bind:checked={comments} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Реакции">
		{#snippet control()}
			<Switch bind:checked={reactions} />
		{/snippet}
	</SettingsRow>
	<Hint style="margin-top:10px">
		Применяется к кругам, в которые вы войдёте потом. Уже настроенные круги не трогаются.
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
