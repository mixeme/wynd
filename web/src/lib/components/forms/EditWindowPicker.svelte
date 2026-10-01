<script lang="ts">
	import NumberField from '$ui/forms/NumberField.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import type { EditWindowKey } from '$lib/circles/settings';

	// Выбор окна правок (2.4, 6.1; план 47, 2.10): шесть чипов в два ряда и
	// «Своё…» — поле часов. Был скопирован в «Новый круг» и настройки круга.
	let {
		value,
		customHours = $bindable(24),
		onpick,
		oncustomchange
	}: {
		value: EditWindowKey;
		customHours?: number;
		onpick: (key: EditWindowKey) => void;
		/** Поле «часов» изменили — настройки круга сохраняют сразу. */
		oncustomchange?: () => void;
	} = $props();

	const firstRow: [EditWindowKey, string][] = [
		['chronicle', 'Летопись'],
		['10m', '10 мин'],
		['1h', 'Час'],
		['1d', 'Сутки']
	];
	const secondRow: [EditWindowKey, string][] = [
		['unlimited', 'Без ограничения'],
		['custom', 'Своё…']
	];
</script>

<ChipGroup>
	{#each firstRow as [key, label] (key)}
		<Chip selected={value === key} onclick={() => onpick(key)}>{label}</Chip>
	{/each}
</ChipGroup>
<ChipGroup>
	{#each secondRow as [key, label] (key)}
		<Chip selected={value === key} onclick={() => onpick(key)}>{label}</Chip>
	{/each}
</ChipGroup>
{#if value === 'custom'}
	<NumberField
		bind:value={customHours}
		min={1}
		max={8760}
		onchange={() => oncustomchange?.()}
		unit="часов"
	/>
{/if}
