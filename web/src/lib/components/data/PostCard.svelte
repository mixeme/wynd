<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		author,
		text,
		media,
		reactions,
		comments,
		headerRight,
		queued = false,
		onclick,
		class: className = '',
		style = '',
		children
	}: {
		author?: Snippet;
		text?: Snippet;
		media?: Snippet;
		reactions?: Snippet;
		comments?: Snippet;
		headerRight?: Snippet;
		queued?: boolean;
		onclick?: () => void;
		class?: string;
		style?: string;
		children?: Snippet;
	} = $props();
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="post {queued ? 'q' : ''} {className}" {style} {onclick}>
	{#if author || headerRight}
		<div class="pa">
			{@render author?.()}
			{@render headerRight?.()}
		</div>
	{/if}
	{#if text}
		<div class="pt">
			{@render text()}
		</div>
	{/if}
	{@render media?.()}
	{@render reactions?.()}
	{@render comments?.()}
</div>
