<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		onclick
	}: {
		children: Snippet;
		onclick: () => void;
	} = $props();
</script>

<button type="button" class="cm" {onclick}>
	{@render children()}
</button>
