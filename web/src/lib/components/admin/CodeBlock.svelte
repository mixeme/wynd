<script lang="ts">
	import type { Snippet } from 'svelte';

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

<div class="codeblk {className}" {style}>
	{@render children()}
</div>
