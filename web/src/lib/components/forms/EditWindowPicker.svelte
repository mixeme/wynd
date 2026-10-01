<script lang="ts">
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Input from '$ui/forms/Input.svelte';
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
	<div class="rowin" style="margin-top:10px;align-items:center">
		<Input
			active
			type="number"
			min="1"
			max="8760"
			bind:value={customHours}
			onchange={() => oncustomchange?.()}
			style="width:72px;margin:0"
		/>
		<span class="hint m-0">часов</span>
	</div>
{/if}
