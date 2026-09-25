<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		selected = false,
		disabled = false,
		onclick,
		class: className = ''
	}: {
		children: Snippet;
		selected?: boolean;
		disabled?: boolean;
		onclick?: () => void;
		class?: string;
	} = $props();
</script>

{#if onclick && !disabled}
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
	<span class="chip {className}" class:on={selected} class:off={disabled} aria-disabled={disabled}>
		{@render children()}
	</span>
{/if}
