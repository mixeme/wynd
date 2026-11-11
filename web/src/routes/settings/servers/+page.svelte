<script lang="ts">
	import { ADD_SERVER_IN_BROWSER } from '$lib/session/addServer';
	import { goUp } from '$lib/navigation/up';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { WORD, plural } from '$lib/format/plural';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import Button from '$ui/forms/Button.svelte';
	import { logoutSession } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { fetchCircles } from '$lib/circles/circles';
	import { loadSessions, removeSession } from '$lib/session/session.svelte';
	import type { SessionRecord } from '$lib/idb/db';
	import { formatSessionDay } from '$lib/format/time';
	import { stopSync } from '$lib/sync/sync';
	import { savePendingEmailChange } from '$lib/auth/emailChange';
	import { savePendingLeave } from '$lib/auth/leaveServer';
	import { loadServerCard, type ServerCardState } from '$lib/instance/serverCard';
	import { desktopBridge } from '$lib/desktop/bridge';

	interface ServerRowState {
		session: SessionRecord;
		host: string;
		circles?: number;
		card: ServerCardState;
		/** Значок «GDPR» у имени сервера (1.2). */
		gdpr?: boolean;
	}

	const desktop = desktopBridge();
	let rows = $state<ServerRowState[]>([]);
	let loading = $state(true);
	let interfaceName = $state(desktop ? displayHost(desktop.origin) : '');

	function pluralCircles(n: number): string {
		return plural(n, WORD.circle);
	}

	function accountSubtitle(session: SessionRecord, circles: number | undefined): string {
		const base = circles === undefined ? 'Число кругов неизвестно' : pluralCircles(circles);
		if (!session.signed_in_at) return base;
		const day = formatSessionDay(session.signed_in_at);
		return day ? `${base} · вошли ${day}` : base;
	}

	async function refresh() {
		const sessions = await loadSessions();
		if (!sessions.length && desktop) {
			const card = await loadServerCard(desktop.origin);
			interfaceName = card.info?.name || displayHost(desktop.origin);
		}
		rows = await Promise.all(
			sessions.map(async (session) => {
				const [list, card] = await Promise.all([
					fetchCircles(session.origin).catch(() => undefined),
					loadServerCard(session.origin)
				]);
				return {
					session,
					host: displayHost(session.origin),
					circles: list?.filter((c) => c.status === 'active').length,
					gdpr: card.info?.allow_name_erasure,
					card
				};
			})
		);
		loading = false;
	}

	async function logout(origin: string) {
		stopSync(origin);
		await logoutSession(origin);
		await removeSession(origin);
		// Другие серверы остаются — список по-прежнему про них. Вышел
		// последний — экран входа, как ждёт владелец, а не пустые «Серверы»
		// (план 46, C80).
		if (!(await loadSessions()).length) {
			await goto('/');
			return;
		}
		await refresh();
	}

	function changeEmail(row: ServerRowState) {
		savePendingEmailChange({
			origin: row.session.origin,
			serverName: row.session.name,
			host: row.host,
			currentEmail: row.session.email,
			email: ''
		});
		void goto('/settings/servers/email');
	}

	function leave(row: ServerRowState) {
		savePendingLeave({
			origin: row.session.origin,
			serverName: row.session.name,
			host: row.host,
			email: row.session.email
		});
		void goto('/settings/servers/leave');
	}

	function addServer() {
		void goto('/join?from=servers');
	}

	onMount(() => {
		void refresh();
	});
</script>

<FormLayout shell app title="Серверы" onback={() => goUp('/settings')}>
	{#if loading}
		<Loading />
	{:else if rows.length === 0}
		<!-- Спокойная констатация, не заголовок: обычный кегль, цвет подсказки. -->
		{#if ADD_SERVER_IN_BROWSER}
			<Hint centered class="mt-28 sz-15">Вы ни на одном сервере не вошли</Hint>
			<Hint centered class="mt-16"
				>Интерфейс продолжит открываться с сервера «{interfaceName}».</Hint
			>
			<Button onclick={addServer}>Войти на сервер</Button>
		{:else}
			<Hint centered class="mt-28 sz-15">На этом устройстве вы ни на одном сервере</Hint>
			<!-- C69: другой сервер отсюда не добавить — входят на этот. -->
			<Button onclick={() => goto('/')}>Войти</Button>
		{/if}
	{:else}
		{#each rows as row, i (row.session.origin + row.session.email)}
			<ServerRow
				name={row.card.info?.name || row.session.name || row.host}
				subtitle={row.host}
				card
				gdpr={row.gdpr}
				connection={row.card.connection}
				top={i === 0 ? 10 : 26}
			/>
			<SettingsRow
				title={row.session.email}
				subtitle={accountSubtitle(row.session, row.circles)}
				chevron={false}
			/>
			<SettingsRow title="Сменить почту" onclick={() => changeEmail(row)} />
			<!-- 7.18: круг приезжает на тот сервер, под которым стоит строка. -->
			<SettingsRow
				title="Принять круг"
				onclick={() =>
					goto(`/settings/servers/accept?origin=${encodeURIComponent(row.session.origin)}`)}
			/>
			<SettingsRow
				icon="out"
				title="Выйти на этом устройстве"
				chevron={false}
				onclick={() => void logout(row.session.origin)}
			/>
			<!-- Уходят с одного сервера: строка под каждым (7.13). -->
			<SettingsRow title="Уйти с сервера насовсем" onclick={() => leave(row)} />
		{/each}
		<Hint class="mt-24">
			Общего профиля на все серверы нет: каждый сервер — отдельное место, даже с той же почтой.
		</Hint>
		{#if ADD_SERVER_IN_BROWSER}
			<Button onclick={addServer}>Войти на сервер</Button>
		{/if}
	{/if}
</FormLayout>
