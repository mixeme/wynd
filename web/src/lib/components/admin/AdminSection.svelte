<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		children,
		class: className = '',
		style = ''
	}: {
		title?: string;
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="adm {className}" {style}>
	{#if title}
		<h4>{title}</h4>
	{/if}
	{@render children()}
</div>
