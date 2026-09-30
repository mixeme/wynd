<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { WORD, plural } from '$lib/format/plural';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
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

	interface ServerRowState {
		session: SessionRecord;
		host: string;
		circles: number;
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
			try {
				circles = (await fetchCircles(session.origin)).filter((c) => c.status === 'active')
					.length;
			} catch {
				/* unreachable origin */
			}
			next.push({
				session,
				host: displayHost(session.origin),
				circles
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

	onMount(() => {
		void refresh();
	});
</script>

<FormLayout shell app title="Серверы" onback={() => goto('/settings')}>
	{#if loading}
		<Loading />
	{:else if rows.length === 0}
		<Hint>На этом устройстве вы ни на одном сервере.</Hint>
	{:else}
		{#each rows as row (row.session.origin + row.session.email)}
			<SectionLabel raw>
				<span style="color:var(--ink)">{row.session.name}</span> · {row.host}
			</SectionLabel>
			<SettingsRow
				title={row.session.email}
				subtitle={accountSubtitle(row.session, row.circles)}
				chevron={false}
				style="padding-top:2px"
			/>
			<SettingsRow
				icon="out"
				title="Выйти с этого сервера"
				chevron={false}
				onclick={() => void logout(row.session.origin)}
			/>
		{/each}
	{/if}

	<Hint style="margin-top:24px">
		Вы выходите с сервера, не из Wynd. Общего профиля на все серверы нет: даже одна почта
		на двух серверах — это два разных места, связать их нельзя.
	</Hint>
	<Button onclick={() => goto('/join?from=servers')}>Добавить сервер</Button>
	<Hint>По ссылке или прямо по адресу — если сервер открыт для новых. Закрытый попросит приглашение.</Hint>
</FormLayout>
