<script lang="ts">
	import type { Snippet } from 'svelte';

	// Список строк MarkRow (план 54): последняя — без черты снизу.
	let {
		children,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="ins-rows {className}" {style}>
	{@render children()}
</div>

<style>
	.ins-rows {
		margin-bottom: 6px;
	}
</style>
