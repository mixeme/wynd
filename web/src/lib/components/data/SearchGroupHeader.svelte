<script lang="ts">
	let {
		color,
		name,
		count,
		class: className = '',
		style = ''
	}: {
		color: string;
		name: string;
		count?: number | string;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="gh2 {className}" {style}>
	<u style:background={color}></u>
	<span>{name}</span>
	{#if count !== undefined}
		<span class="cnt2">{count}</span>
	{/if}
</div>
