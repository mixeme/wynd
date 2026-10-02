<script lang="ts">
	// Волна голосового (4.23, 4.24, 4.27): столбики уровней 0–100. Прослушанное
	// — цветом круга, остальное — линией; во время записи вся волна цветная.
	let {
		peaks,
		progress = 1,
		class: className = ''
	}: {
		peaks: number[];
		/** Доля прослушанного, 0–1. */
		progress?: number;
		class?: string;
	} = $props();

	const played = $derived(Math.round(progress * peaks.length));
</script>

<span class="vwave {className}" aria-hidden="true">
	{#each peaks as p, i (i)}
		<i class:off={i >= played} style:height="{Math.max(12, p)}%"></i>
	{/each}
</span>
