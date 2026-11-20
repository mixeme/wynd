<script lang="ts">
	// Волна голосового (4.12, 4.13, 4.16): столбики уровней 0–100. Прослушанное
	// — цветом круга, остальное — линией; во время записи вся волна цветная.
	let {
		peaks,
		progress = 1,
		fill = false,
		class: className = ''
	}: {
		peaks: number[];
		/** Доля прослушанного, 0–1. */
		progress?: number;
		/** Волна в карточке доходит до длительности, в том числе у коротких записей. */
		fill?: boolean;
		class?: string;
	} = $props();

	const minBars = 48;
	const bars = $derived(
		fill && peaks.length < minBars
			? Array.from(
					{ length: minBars },
					(_, index) => peaks[Math.floor((index * peaks.length) / minBars)] ?? 20
				)
			: peaks
	);
	const played = $derived(Math.round(progress * bars.length));
</script>

<span class="vwave {className}" class:vwave-fill={fill} aria-hidden="true">
	{#each bars as p, i (i)}
		<i class:off={i >= played} style:height="{Math.max(12, p)}%"></i>
	{/each}
</span>
