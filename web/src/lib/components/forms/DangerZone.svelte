<script lang="ts">
	let {
		items,
		label = 'Необратимо',
		onitem,
		class: className = '',
		style
	}: {
		items: string[];
		label?: string;
		onitem?: (item: string) => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="danger {className}" {style}>
	<div class="dl">{label}</div>
	{#each items as item (item)}
		{#if onitem}
			<button type="button" class="di" onclick={() => onitem(item)}>{item}</button>
		{:else}
			<div class="di">{item}</div>
		{/if}
	{/each}
</div>
