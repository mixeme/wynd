<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label,
		children,
		class: className = '',
		style = ''
	}: {
		label?: string;
		children?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="mnth {className}" {style}>
	{#if children}
		{@render children()}
	{:else}
		{label}
	{/if}
</div>
