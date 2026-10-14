<script lang="ts">
	// Легенда шкалы хранилища (9.5): круги и готовые архивы отдельно. Образец
	// цвета — полоска, как отрезок на шкале.
	let { items }: { items: { label: string; swatch: string }[] } = $props();
</script>

<div class="stack-legend">
	{#each items as item (item.label)}
		<span><span class="stack-legend-swatch" style:background={item.swatch}></span>{item.label}</span>
	{/each}
</div>
