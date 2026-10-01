<script lang="ts">
	import type { Snippet } from 'svelte';

	// Строка формы админки «подпись · поле · единица» (план 47, 2.18; кадры
	// 9.x): подпись 12,5 px постоянной ширины, чтобы поля стояли столбцом.
	let {
		label,
		width = 120,
		unit,
		children,
		class: className = ''
	}: {
		label: string;
		/** Ширина подписи: 88 — короткие («Хост», «Порт»), 120 — длинные. */
		width?: 88 | 120;
		/** Единица или пояснение после поля: «px», «587 или 465». */
		unit?: string;
		children: Snippet;
		class?: string;
	} = $props();
</script>

<div class="flex-mid gap-10 {className}">
	<span class="sz-12 nowrap flex-fix" class:w88={width === 88} class:w120={width === 120}
		>{label}</span
	>
	{@render children()}
	{#if unit}
		<span class="note nowrap">{unit}</span>
	{/if}
</div>
