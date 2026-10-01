<script lang="ts">
	// Полоса заполнения: место круга (6.2, 6.10), кэш, отправка записи. inline —
	// короткая полоска в строке таблицы (квота круга в админке, 9.5; план 47,
	// 2.15), цвет — свой у каждой строки.
	let {
		value,
		max = 100,
		inline = false,
		color
	}: {
		value: number;
		max?: number;
		inline?: boolean;
		/** Цвет заполнения; по умолчанию — цвет круга. */
		color?: string;
	} = $props();

	const width = $derived(`${Math.min(100, Math.max(0, (value / max) * 100))}%`);
</script>

{#if inline}
	<span class="qbar"><u style:width={width} style:background={color}></u></span>
{:else}
	<div class="meter">
		<u style:width={width} style:background={color}></u>
	</div>
{/if}
