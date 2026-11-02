<script lang="ts">
	import { onMount } from 'svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import { loadSessions } from '$lib/session/session.svelte';
	import { displayHost } from '$lib/auth/origin';
	import { authErrorHint } from '$lib/auth/auth';
	import { loadServerCard, type ServerCardState } from '$lib/instance/serverCard';
	import type { SessionRecord } from '$lib/idb/db';
	import {
		fetchReviveRoom,
		roomCaption,
		PlaceWithdrawalError,
		type ReviveRoom
	} from '$lib/archive/revive';
	import { formatBytes } from '$lib/format/bytes';
	let {
		origin,
		onselect,
		onback
	}: {
		origin: string;
		onselect: (session: SessionRecord) => Promise<void>;
		onback: () => void;
	} = $props();
	let sessions = $state<SessionRecord[]>([]);
	let cards = $state<Record<string, ServerCardState>>({});
	let rooms = $state<Record<string, ReviveRoom>>({});
	let busy = $state(false);
	let error = $state('');
	onMount(async () => {
		sessions = await loadSessions();
		await Promise.all(
			sessions.map(async (s) => {
				await Promise.all([
					loadServerCard(s.origin).then((card) => {
						cards[s.origin] = card;
					}),
					fetchReviveRoom(s.origin, 0)
						.then((room) => {
							rooms[s.origin] = room;
						})
						.catch(() => {})
				]);
			})
		);
	});
	async function choose(session: SessionRecord) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await onselect(session);
		} catch (err) {
			error = err instanceof PlaceWithdrawalError ? err.message : authErrorHint(err);
		} finally {
			busy = false;
		}
	}
</script>

<FormLayout
	shell
	app
	title="Сервер"
	onback={() => {
		if (!busy) onback();
	}}
>
	<Hint
		>При выборе другого сервера прежний запрос на место будет отозван. Ссылка или выбранный архив
		сохранятся.</Hint
	>
	<Label>Ваши серверы</Label>
	{#each sessions as session (session.origin)}
		<ServerRow
			name={cards[session.origin]?.info?.name || session.name || displayHost(session.origin)}
			subtitle={displayHost(session.origin)}
			detail={rooms[session.origin] ? roomCaption(rooms[session.origin], formatBytes) : undefined}
			variant={session.origin === origin ? 'ok' : undefined}
			gdpr={cards[session.origin]?.info?.allow_name_erasure}
			connection={cards[session.origin] ? cards[session.origin].connection : 'checking'}
			onclick={() => void choose(session)}
		/>
	{/each}
	{#if busy}<Hint>Проверяем сервер…</Hint>{/if}
	{#if error}<Hint class="danger">{error}</Hint>{/if}
</FormLayout>
