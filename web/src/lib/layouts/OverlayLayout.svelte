<script lang="ts">
	import Dialog from '$ui/overlays/Dialog.svelte';
	import Scrim from '$ui/overlays/Scrim.svelte';
	import Sheet from '$ui/overlays/Sheet.svelte';
	import type { Snippet } from 'svelte';

	let {
		variant = 'sheet',
		scrim = true,
		grip = true,
		class: className = '',
		style = '',
		children
	}: {
		variant?: 'sheet' | 'dialog';
		scrim?: boolean;
		grip?: boolean;
		class?: string;
		style?: string;
		children: Snippet;
	} = $props();
</script>

{#if scrim}
	<Scrim />
{/if}
{#if variant === 'dialog'}
	<Dialog class={className} {style}>{@render children()}</Dialog>
{:else}
	<Sheet {grip} class={className} {style}>{@render children()}</Sheet>
{/if}
