<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		aside,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		/** Мелко справа, без капители: «необязательно» (план 47, 3.3). */
		aside?: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="lab {aside ? 'label-row' : ''} {className}" {style}>
	{#if aside}
		<span>{@render children()}</span>
		<span class="label-aside">{aside}</span>
	{:else}
		{@render children()}
	{/if}
</div>
