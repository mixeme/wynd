<script lang="ts">
	import Row from '$ui/data/Row.svelte';
	import type { Snippet } from 'svelte';
	import { quoteMatch } from '$lib/journal/search';

	let {
		author,
		time,
		preview,
		kind,
		snippet = '',
		query = '',
		thumb,
		thumbUrl,
		thumbVariant,
		onclick,
		class: className = '',
		style = ''
	}: {
		author: string;
		time: string;
		/** Своё превью; без него — по kind из snippet и query (2.9, 3.x). */
		preview?: Snippet;
		/** Что нашлось: `post`, `comment` (« · комментарий») или `day` («день»). */
		kind?: string;
		snippet?: string;
		/** Запрос — найденное слово берётся в ёлочки. */
		query?: string;
		thumb?: boolean;
		thumbUrl?: string;
		thumbVariant?: string;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<Row {onclick} class={className} {style}>
	{#snippet main()}
		<div style="font-weight:600">
			{author}<span style="font-weight:400;color:var(--faint);font-size:11.5px"> · {time}</span>
		</div>
		<div class="sub">
			{#if preview}
				{@render preview()}
			{:else if kind === 'day'}
				<span class="faint">день</span>
			{:else}
				{quoteMatch(snippet, query)}
				{#if kind === 'comment'}
					<span class="faint"> · комментарий</span>
				{/if}
			{/if}
		</div>
	{/snippet}
	{#snippet trailing()}
		{#if thumb}
			{#if thumbUrl}
				<div class="thumb" style="background-image:url({thumbUrl});background-size:cover;background-position:center"></div>
			{:else}
				<div class="thumb pic {thumbVariant ?? 'p1'}"></div>
			{/if}
		{/if}
	{/snippet}
</Row>
