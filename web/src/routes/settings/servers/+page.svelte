<script lang="ts">
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
	import { fetchInstance } from '$lib/auth/auth';

	interface ServerRowState {
		session: SessionRecord;
		host: string;
		circles: number;
		/** Значок «GDPR» у имени сервера (1.10). */
		gdpr: boolean;
	}

	let rows = $state<ServerRowState[]>([]);
	let loading = $state(true);

	function pluralCircles(n: number): string {
		return plural(n, WORD.circle);
	}

	function accountSubtitle(session: SessionRecord, circles: number): string {
		const base = pluralCircles(circles);
		if (!session.signed_in_at) return base;
		const day = formatSessionDay(session.signed_in_at);
		return day ? `${base} · вошли ${day}` : base;
	}

	async function refresh() {
		const sessions = await loadSessions();
		const next: ServerRowState[] = [];
		for (const session of sessions) {
			let circles = 0;
			let gdpr = false;
			try {
				const [list, instance] = await Promise.all([
					fetchCircles(session.origin),
					fetchInstance(session.origin).catch(() => undefined)
				]);
				circles = list.filter((c) => c.status === 'active').length;
				gdpr = Boolean(instance?.allow_name_erasure);
			} catch {
				/* unreachable origin */
			}
			next.push({
				session,
				host: displayHost(session.origin),
				circles,
				gdpr
			});
		}
		rows = next;
		loading = false;
	}

	async function logout(origin: string) {
		stopSync(origin);
		await logoutSession(origin);
		await removeSession(origin);
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

	onMount(() => {
		void refresh();
	});
</script>

<FormLayout shell app title="Серверы" onback={() => goUp('/settings')}>
	{#if loading}
		<Loading />
	{:else if rows.length === 0}
		<Hint>На этом устройстве вы ни на одном сервере.</Hint>
	{:else}
		{#each rows as row, i (row.session.origin + row.session.email)}
			<ServerRow
				name={row.session.name || row.host}
				subtitle={row.host}
				card
				gdpr={row.gdpr}
				top={i === 0 ? 10 : 26}
			/>
			<SettingsRow
				title={row.session.email}
				subtitle={accountSubtitle(row.session, row.circles)}
				chevron={false}
			/>
			<SettingsRow title="Сменить почту" onclick={() => changeEmail(row)} />
			<!-- 7.15: круг приезжает на тот сервер, под которым стоит строка. -->
			<SettingsRow
				title="Принять круг"
				onclick={() => goto(`/settings/servers/accept?origin=${encodeURIComponent(row.session.origin)}`)}
			/>
			<SettingsRow
				icon="out"
				title="Выйти на этом устройстве"
				chevron={false}
				onclick={() => void logout(row.session.origin)}
			/>
			<!-- Уходят с одного сервера: строка под каждым (7.33). -->
			<SettingsRow title="Уйти с сервера насовсем" onclick={() => leave(row)} />
		{/each}
	{/if}

	<Hint class="mt-24">
		Вы выходите с сервера, не из Wynd. Общего профиля на все серверы нет: даже одна почта
		на двух серверах — это два разных места, связать их нельзя.
	</Hint>
	<Button onclick={() => goto('/join?from=servers')}>Добавить сервер</Button>
	<Hint>По ссылке или прямо по адресу — если сервер открыт для новых. Закрытый попросит приглашение.</Hint>
</FormLayout>
