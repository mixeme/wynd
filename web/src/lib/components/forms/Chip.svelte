<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		selected = false,
		onclick,
		class: className = ''
	}: {
		children: Snippet;
		selected?: boolean;
		onclick?: () => void;
		class?: string;
	} = $props();
</script>

{#if onclick}
	<button
		type="button"
		class="chip {className}"
		class:on={selected}
		aria-pressed={selected}
		{onclick}
	>
		{@render children()}
	</button>
{:else}
	<span class="chip {className}" class:on={selected}>
		{@render children()}
	</span>
{/if}
