<!-- Smoke-test: FormLayout from docs/screens.html #e2-4 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import ColorSwatches from '$ui/forms/ColorSwatches.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import type { CircleColor } from '$lib/theme/colors';

	let color = $state<CircleColor>('ochre');
</script>

<FormLayout {color} title="Новый круг">
	<Label>Сервер</Label>
	<ServerRow
		name="Дом Ани"
		subtitle="home.example.org · ещё 5 ваших кругов"
		variant="select"
		style="padding:2px 16px 10px"
	/>
	<Hint style="margin-top:0">
		Круг будет жить здесь. Если потребуется, его можно перенести на другой сервер. Сервер
		хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
		<a class="under" href="https://github.com/mixeme/wynd">поднимите свой</a>.
	</Hint>
	<Label>Название</Label>
	<FieldDisplay value="Дача" active />
	<Label>Цвет</Label>
	<ColorSwatches bind:value={color} />
	<Label>Окно правок</Label>
	<ChipGroup>
		<Chip>Летопись</Chip>
		<Chip>10 мин</Chip>
		<Chip selected>Час</Chip>
		<Chip>Сутки</Chip>
	</ChipGroup>
	<ChipGroup style="margin-top:8px">
		<Chip>Без ограничения</Chip>
		<Chip>Своё…</Chip>
	</ChipGroup>
	<Hint>
		Сколько времени после публикации запись можно править. «Летопись» — набело и навсегда.
		Правило меняется потом, но подействует только на новые записи.
	</Hint>
	<Button variant="colored" onclick={() => {}}>Создать и позвать</Button>
	<Hint centered style="margin-top:26px">
		<TextButton onclick={() => {}}>Или круг только для себя, как дневник</TextButton>
	</Hint>
</FormLayout>
