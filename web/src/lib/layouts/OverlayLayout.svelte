<script lang="ts">
	import Dialog from '$ui/overlays/Dialog.svelte';
	import Scrim from '$ui/overlays/Scrim.svelte';
	import Sheet from '$ui/overlays/Sheet.svelte';
	import type { Snippet } from 'svelte';

	let {
		variant = 'sheet',
		scrim = true,
		grip = true,
		ondismiss,
		class: className = '',
		style = '',
		children
	}: {
		variant?: 'sheet' | 'dialog';
		scrim?: boolean;
		grip?: boolean;
		ondismiss?: () => void;
		class?: string;
		style?: string;
		children: Snippet;
	} = $props();
</script>

{#if scrim}
	<Scrim onclick={ondismiss} />
{/if}
{#if variant === 'dialog'}
	<Dialog class={className} {style}>{@render children()}</Dialog>
{:else}
	<Sheet {grip} class={className} {style}>{@render children()}</Sheet>
{/if}
