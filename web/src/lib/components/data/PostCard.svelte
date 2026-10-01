<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { Action } from 'svelte/action';

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

	const bindOpen: Action<HTMLElement, (() => void) | undefined> = (node, fn) => {
		let current = fn;
		function onClick(e: MouseEvent) {
			if (!current) return;
			const target = e.target as HTMLElement;
			if (target.closest('button, a, input, textarea, select, label, .rxpick')) return;
			current();
		}
		node.addEventListener('click', onClick);
		return {
			update(next) {
				current = next;
			},
			destroy() {
				node.removeEventListener('click', onClick);
			}
		};
	};
</script>

<div class="post {queued ? 'q' : ''} {className}" {style} use:bindOpen={onclick}>
	{#if author || headerRight}
		<div class="pa">
			{@render author?.()}
			{@render headerRight?.()}
		</div>
	{/if}
	{#if text}
		<!-- В одну строку: с pre-wrap перенос разметки стал бы пустой строкой. -->
		<div class="pt">{@render text()}</div>
	{/if}
	{@render media?.()}
	{@render reactions?.()}
	{@render comments?.()}
</div>
