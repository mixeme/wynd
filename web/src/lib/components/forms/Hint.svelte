<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		centered = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		centered?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="hint {className}" class:ctr={centered} {style}>
	{@render children()}
</div>
