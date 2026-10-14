<script lang="ts">
	let {
		items,
		label = 'Необратимо',
		onitem,
		disabled = false,
		class: className = '',
		style
	}: {
		items: string[];
		label?: string;
		onitem?: (item: string) => void;
		/** Правило видно, но действие недоступно (6.19, 7.30): блок погашен и не нажимается. */
		disabled?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="danger {className}" class:off={disabled} aria-disabled={disabled || undefined} {style}>
	<div class="dl">{label}</div>
	{#each items as item (item)}
		{#if onitem && !disabled}
			<button type="button" class="di" onclick={() => onitem(item)}>{item}</button>
		{:else}
			<div class="di">{item}</div>
		{/if}
	{/each}
</div>
