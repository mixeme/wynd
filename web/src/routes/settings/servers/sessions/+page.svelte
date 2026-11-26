<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { navigateBack } from '$lib/navigation/up';
	import { displayHost } from '$lib/auth/origin';
	import { formatSessionDay, formatSessionSeen } from '$lib/format/time';
	import { fetchSessions, revokeSession, type SessionItem } from '$lib/auth/sessions';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Button from '$ui/forms/Button.svelte';

	// У сервера на своём адресе origin — пустая строка: не проверять как истину.
	let origin = $state($page.url.searchParams.get('origin') ?? '');

	let sessions = $state<SessionItem[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busyId = $state('');
	let busyOthers = $state(false);

	async function load() {
		error = '';
		try {
			sessions = await fetchSessions(origin);
		} catch {
			error = 'Не удалось загрузить сессии';
		} finally {
			loading = false;
		}
	}

	function subtitleFor(item: SessionItem): string {
		const seen = formatSessionSeen(item.last_seen_at);
		const expiry = item.expires_at ? `истечёт ${formatSessionDay(item.expires_at)}` : '';
		if (item.current) {
			return expiry ? `Это устройство · ${seen} · ${expiry}` : `Это устройство · ${seen}`;
		}
		return expiry ? `${seen} · ${expiry}` : seen;
	}

	async function revoke(item: SessionItem) {
		error = '';
		busyId = item.id;
		try {
			await revokeSession(origin, item.id);
			await load();
		} catch {
			error = 'Не удалось завершить сессию';
		} finally {
			busyId = '';
		}
	}

	async function revokeOthers() {
		const others = sessions.filter((s) => !s.current);
		if (!others.length || busyOthers) return;
		error = '';
		busyOthers = true;
		try {
			for (const item of others) {
				await revokeSession(origin, item.id);
			}
			await load();
		} catch {
			error = 'Не удалось завершить сессии';
			await load().catch(() => {});
		} finally {
			busyOthers = false;
		}
	}

	onMount(() => {
		void load();
	});
</script>

<FormLayout shell app title="Сессии" onback={navigateBack}>
	{#if loading}
		<Loading />
	{:else if error && !sessions.length}
		<Hint class="gutter">{error}</Hint>
		<TextButton onclick={() => void load()}>Повторить</TextButton>
	{:else}
		<Hint class="mt-10">
			Где вы вошли на «{displayHost(origin)}». Завершённая сессия при следующем действии попросит
			войти заново — код из письма.
		</Hint>
		{#each sessions as item (item.id)}
			<SettingsRow title={item.device || 'Устройство'} subtitle={subtitleFor(item)} chevron={false}>
				{#snippet control()}
					<!-- Текущая сессия без действия: выход остаётся на «Серверах» (26.11). -->
					{#if !item.current}
						<TextButton
							loading={busyId === item.id}
							disabled={busyId !== '' && busyId !== item.id}
							onclick={() => void revoke(item)}
						>
							Завершить
						</TextButton>
					{/if}
				{/snippet}
			</SettingsRow>
		{/each}
		{#if error}
			<Hint class="gutter">{error}</Hint>
		{/if}
		{#if sessions.length > 1}
			<Button loading={busyOthers} onclick={() => void revokeOthers()}>Завершить прочие</Button>
		{/if}
	{/if}
</FormLayout>
