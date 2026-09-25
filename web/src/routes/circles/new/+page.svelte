<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import ColorSwatches from '$ui/forms/ColorSwatches.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import {
		createCircle,
		fetchCircles,
		ownerNameFromSession,
		type CircleColor
	} from '$lib/circles/circles';
	import { setCircleColor } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { displayHost } from '$lib/auth/origin';
	import { authErrorHint } from '$lib/auth/auth';
	import { loadSessions } from '$lib/session/session.svelte';
	import type { SessionRecord } from '$lib/idb/db';

	type EditWindow = 'chronicle' | '10m' | '1h' | '1d' | 'unlimited' | 'custom';

	let sessions = $state<SessionRecord[]>([]);
	let selectedOrigin = $state('');
	let name = $state('');
	let color = $state<CircleColor>('ochre');
	let editWindow = $state<EditWindow>('1h');
	let customHours = $state(24);
	let diaryMode = $state(false);
	let loading = $state(false);
	let error = $state('');
	let circleCounts = $state<Record<string, number>>({});

	onMount(async () => {
		sessions = await loadSessions();
		if (!sessions.length) {
			goto('/');
			return;
		}
		selectedOrigin = sessions[0].origin;
		const counts: Record<string, number> = {};
		for (const session of sessions) {
			try {
				const list = await fetchCircles(session.origin);
				counts[session.origin] = list.length;
			} catch {
				counts[session.origin] = 0;
			}
		}
		circleCounts = counts;
	});

	const selectedSession = $derived(sessions.find((s) => s.origin === selectedOrigin));

	function editWindowSec(): number | null {
		switch (editWindow) {
			case 'chronicle':
				return 0;
			case '10m':
				return 600;
			case '1h':
				return 3600;
			case '1d':
				return 86400;
			case 'custom':
				return Math.max(1, Math.min(8760, customHours)) * 3600;
			default:
				return null;
		}
	}

	function toggleDiaryMode() {
		diaryMode = !diaryMode;
		if (diaryMode) editWindow = 'chronicle';
	}

	async function onSubmit() {
		error = '';
		const trimmed = name.trim();
		if (!trimmed || !selectedSession) {
			error = 'Введите название круга';
			return;
		}
		loading = true;
		try {
			const created = await createCircle(selectedSession.origin, {
				name: trimmed,
				owner_name: ownerNameFromSession(selectedSession),
				edit_window_sec: editWindowSec(),
				color
			});
			await setCircleColor(selectedSession.origin, created.id, color);
			rememberCircleOrigin(created.id, selectedSession.origin);
			if (diaryMode) goto(`/circles/${created.id}`);
			else goto(`/circles/${created.id}/settings/invite?from=create`);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	function serverSubtitle(session: SessionRecord): string {
		const count = circleCounts[session.origin] ?? 0;
		const host = displayHost(session.origin);
		const tail = count ? ` · ещё ${count} ваших кругов` : '';
		return `${host}${tail}`;
	}
</script>

<FormLayout {color} app title="Новый круг" onback={() => goto('/circles')}>
	<Label>Сервер</Label>
	{#each sessions as session (session.origin)}
		<ServerRow
			name={session.name}
			subtitle={serverSubtitle(session)}
			variant={session.origin === selectedOrigin ? 'select' : 'info'}
			style="padding:2px 16px 10px"
			class={session.origin === selectedOrigin ? '' : 'dim'}
			onclick={() => (selectedOrigin = session.origin)}
		/>
	{/each}
	<Hint style="margin-top:0">
		Круг будет жить здесь. Если потребуется, его можно перенести на другой сервер. Сервер
		хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
		<a class="under" href="https://github.com/mixeme/wynd">поднимите свой</a>.
	</Hint>
	<Label>Название</Label>
	<Input active type="text" bind:value={name} />
	<Label>Цвет</Label>
	<ColorSwatches bind:value={color} />
	<Label>Окно правок</Label>
	<ChipGroup>
		<Chip selected={editWindow === 'chronicle'} onclick={() => (editWindow = 'chronicle')}>
			Летопись
		</Chip>
		<Chip selected={editWindow === '10m'} onclick={() => (editWindow = '10m')}>10 мин</Chip>
		<Chip selected={editWindow === '1h'} onclick={() => (editWindow = '1h')}>Час</Chip>
		<Chip selected={editWindow === '1d'} onclick={() => (editWindow = '1d')}>Сутки</Chip>
	</ChipGroup>
	<ChipGroup style="margin-top:8px">
		<Chip selected={editWindow === 'unlimited'} onclick={() => (editWindow = 'unlimited')}>
			Без ограничения
		</Chip>
		<Chip selected={editWindow === 'custom'} onclick={() => (editWindow = 'custom')}>Своё…</Chip>
	</ChipGroup>
	{#if editWindow === 'custom'}
		<div class="rowin" style="margin-top:10px;align-items:center">
			<Input
				active
				type="number"
				min="1"
				max="8760"
				bind:value={customHours}
				style="width:72px;margin:0"
			/>
			<span class="hint" style="margin:0">часов</span>
		</div>
	{/if}
	<Hint>
		Сколько времени после публикации запись можно править. «Летопись» — набело и навсегда.
		Правило меняется потом, но подействует только на новые записи.
	</Hint>
	<Button variant="colored" {loading} onclick={onSubmit}>
		{diaryMode ? 'Завести дневник' : 'Создать и позвать'}
	</Button>
	{#if !diaryMode}
		<Hint centered style="margin-top:26px">
			<TextButton onclick={toggleDiaryMode}>Или круг только для себя, как дневник</TextButton>
		</Hint>
	{:else}
		<Hint centered style="margin-top:26px">
			<TextButton onclick={toggleDiaryMode}>Это дневник · окно «Летопись»</TextButton>
		</Hint>
	{/if}
	{#if error}
		<Hint style="margin-top:12px">{error}</Hint>
	{/if}
</FormLayout>

<style>
	:global(.dim) {
		opacity: 0.55;
	}
</style>
