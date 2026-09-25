<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		raw = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		raw?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="lab {raw ? 'raw' : ''} {className}" {style}>
	{@render children()}
</div>
