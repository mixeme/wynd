<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		text,
		children,
		class: className = '',
		style = ''
	}: {
		text?: string;
		children?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="req {className}" {style}>
	{#if text !== undefined}
		{text}
	{:else if children}
		{@render children()}
	{/if}
</div>
