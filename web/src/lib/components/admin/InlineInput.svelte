<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		mono = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		mono?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<span class="inp {className}" class:mono {style}>
	{@render children()}
</span>
