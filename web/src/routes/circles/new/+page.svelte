<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
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
		ownerNameFromSession
	} from '$lib/circles/circles';
	import { setCircleColor } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { authErrorHint } from '$lib/auth/auth';
	import { loadSourceUrl, sourceUrl } from '$lib/instance/source.svelte';
	import { NEW_CIRCLE_CTX, type NewCircleContext, type NewCircleEditWindow } from '$lib/circles/new-circle';

	const form = getContext<NewCircleContext>(NEW_CIRCLE_CTX);

	const selectedSession = $derived(
		form.sessions.find((s) => s.origin === form.selectedOrigin)
	);

	// Ссылка «поднимите свой» ведёт на исходники выбранного сервера (LIC-2).
	$effect(() => {
		loadSourceUrl(form.selectedOrigin);
	});

	let windowBeforeDiary: NewCircleEditWindow = '1h';

	function editWindowSec(): number | null {
		switch (form.editWindow) {
			case 'chronicle':
				return 0;
			case '10m':
				return 600;
			case '1h':
				return 3600;
			case '1d':
				return 86400;
			case 'custom':
				return Math.max(1, Math.min(8760, form.customHours)) * 3600;
			default:
				return null;
		}
	}

	function toggleDiaryMode() {
		if (!form.diaryMode) {
			windowBeforeDiary = form.editWindow;
			form.diaryMode = true;
			form.editWindow = 'chronicle';
		} else {
			form.diaryMode = false;
			form.editWindow = windowBeforeDiary;
		}
	}

	function openServerPick() {
		goto('/circles/new/server');
	}

	async function onSubmit() {
		form.error = '';
		const trimmed = form.name.trim();
		if (!trimmed || !selectedSession) {
			form.error = 'Введите название круга';
			return;
		}
		form.loading = true;
		try {
			const created = await createCircle(selectedSession.origin, {
				name: trimmed,
				owner_name: ownerNameFromSession(selectedSession),
				edit_window_sec: editWindowSec(),
				color: form.color
			});
			await setCircleColor(selectedSession.origin, created.id, form.color);
			rememberCircleOrigin(created.id, selectedSession.origin);
			if (form.diaryMode) goto(`/circles/${created.id}`);
			else goto(`/circles/${created.id}/settings/invite?from=create`);
		} catch (err) {
			form.error = authErrorHint(err);
		} finally {
			form.loading = false;
		}
	}
</script>

<FormLayout color={form.color} app title="Новый круг" onback={() => goto('/circles')}>
	<Label>Сервер</Label>
	{#if selectedSession}
		<ServerRow
			name={selectedSession.name}
			subtitle={form.serverSubtitle(selectedSession)}
			variant="select"
			style="padding:2px 16px 10px"
			onclick={openServerPick}
		/>
	{/if}
	<Hint style="margin-top:0">
		Круг будет жить здесь. Если потребуется, его можно перенести на другой сервер. Сервер
		хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
		<a class="under" href={sourceUrl(form.selectedOrigin)}>поднимите свой</a>.
	</Hint>
	<Label>Название</Label>
	<Input active type="text" bind:value={form.name} />
	<Label>Цвет</Label>
	<ColorSwatches bind:value={form.color} />
	<Label>Окно правок</Label>
	<ChipGroup>
		<Chip
			selected={form.editWindow === 'chronicle'}
			onclick={() => (form.editWindow = 'chronicle')}
		>
			Летопись
		</Chip>
		<Chip selected={form.editWindow === '10m'} onclick={() => (form.editWindow = '10m')}>
			10 мин
		</Chip>
		<Chip selected={form.editWindow === '1h'} onclick={() => (form.editWindow = '1h')}>Час</Chip>
		<Chip selected={form.editWindow === '1d'} onclick={() => (form.editWindow = '1d')}>Сутки</Chip>
	</ChipGroup>
	<ChipGroup style="margin-top:8px">
		<Chip
			selected={form.editWindow === 'unlimited'}
			onclick={() => (form.editWindow = 'unlimited')}
		>
			Без ограничения
		</Chip>
		<Chip selected={form.editWindow === 'custom'} onclick={() => (form.editWindow = 'custom')}>
			Своё…
		</Chip>
	</ChipGroup>
	{#if form.editWindow === 'custom'}
		<div class="rowin" style="margin-top:10px;align-items:center">
			<Input
				active
				type="number"
				min="1"
				max="8760"
				bind:value={form.customHours}
				style="width:72px;margin:0"
			/>
			<span class="hint" style="margin:0">часов</span>
		</div>
	{/if}
	<Hint>
		{#if form.diaryMode}
			Дневник, который нельзя переписать задним числом, — сильная штука. Поэтому здесь предложена
			«Летопись», но она не обязательна.
		{:else}
			Сколько времени после публикации запись можно править. «Летопись» — набело и навсегда.
			Правило меняется потом, но подействует только на новые записи.
		{/if}
	</Hint>
	<Button variant="colored" loading={form.loading} onclick={onSubmit}>
		{form.diaryMode ? 'Завести дневник' : 'Создать и позвать'}
	</Button>
	{#if !form.diaryMode}
		<Hint centered style="margin-top:26px">
			<TextButton onclick={toggleDiaryMode}>Или круг только для себя, как дневник</TextButton>
		</Hint>
	{:else}
		<Hint centered style="margin-top:26px">
			<TextButton onclick={toggleDiaryMode}>Или круг, куда можно позвать</TextButton>
		</Hint>
	{/if}
	{#if form.error}
		<Hint style="margin-top:12px">{form.error}</Hint>
	{/if}
</FormLayout>
