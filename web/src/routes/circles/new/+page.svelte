<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import EditWindowPicker from '$ui/forms/EditWindowPicker.svelte';
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import ColorSwatches from '$ui/forms/ColorSwatches.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
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

	// Круг заводится на следующем шаге, когда известно имя создателя: так в
	// журнале нового круга нет строки «почта теперь Имя» (1.3 для создателя).
	function onSubmit() {
		form.error = '';
		if (!form.name.trim() || !selectedSession) {
			form.error = 'Введите название круга';
			return;
		}
		goto('/circles/new/you');
	}
</script>

<FormLayout color={form.color} app title="Новый круг" onback={() => goUp('/circles')}>
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
	<Hint class="mt-0">
		Круг будет жить здесь. Если потребуется, его можно перенести на другой сервер. Сервер
		хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
		<a class="under" href={sourceUrl(form.selectedOrigin)}>поднимите свой</a>.
	</Hint>
	<Label>Название</Label>
	<Input active type="text" bind:value={form.name} />
	<Label>Цвет</Label>
	<ColorSwatches bind:value={form.color} />
	<Label>Окно правок</Label>
	<EditWindowPicker
		value={form.editWindow}
		bind:customHours={form.customHours}
		onpick={(key) => (form.editWindow = key)}
	/>
	<Hint>
		{#if form.diaryMode}
			Дневник, который нельзя переписать задним числом, — сильная штука. Поэтому здесь предложена
			«Летопись», но она не обязательна.
		{:else}
			Сколько времени после публикации запись можно править. «Летопись» — набело и навсегда.
			Правило меняется потом, но подействует только на новые записи.
		{/if}
	</Hint>
	<Button variant="colored" onclick={onSubmit}>
		{form.diaryMode ? 'Завести дневник' : 'Создать и позвать'}
	</Button>
	{#if !form.diaryMode}
		<Hint class="mt-26" centered>
			<TextButton onclick={toggleDiaryMode}>Или круг только для себя, как дневник</TextButton>
		</Hint>
	{:else}
		<Hint class="mt-26" centered>
			<TextButton onclick={toggleDiaryMode}>Или круг, куда можно позвать</TextButton>
		</Hint>
	{/if}
	{#if form.error}
		<Hint class="mt-12">{form.error}</Hint>
	{/if}
</FormLayout>
