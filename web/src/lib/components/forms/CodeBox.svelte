<script lang="ts">
	let {
		digits = [],
		length = 6,
		active = 0
	}: {
		digits?: string[];
		length?: number;
		active?: number;
	} = $props();

	const cells = $derived(
		Array.from({ length }, (_, i) => ({
			char: digits[i] ?? '',
			active: i === active
		}))
	);
</script>

<div class="codebox">
	{#each cells as cell, i (i)}
		<u class:act={cell.active}>{cell.char}</u>
	{/each}
</div>
