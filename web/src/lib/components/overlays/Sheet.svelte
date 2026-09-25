<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		grip = true,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		grip?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="sheet {className}" {style}>
	{#if grip}
		<div class="grip"></div>
	{/if}
	{@render children()}
</div>
