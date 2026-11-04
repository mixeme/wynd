<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		selected = false,
		disabled = false,
		color,
		onclick,
		class: className = ''
	}: {
		children: Snippet;
		selected?: boolean;
		disabled?: boolean;
		/** Необязательная цветная метка, например цвет круга в выборе группы. */
		color?: string;
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
		{#if color}<span class="chip-color" style:background-color={color} aria-hidden="true"
			></span>{/if}
		{@render children()}
	</button>
{:else}
	<span class="chip {className}" class:on={selected} class:off={disabled} aria-disabled={disabled}>
		{#if color}<span class="chip-color" style:background-color={color} aria-hidden="true"
			></span>{/if}
		{@render children()}
	</span>
{/if}
