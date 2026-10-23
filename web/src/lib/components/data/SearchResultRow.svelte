<script lang="ts">
	import Row from '$ui/data/Row.svelte';
	import EntryDateMark from '$ui/data/EntryDateMark.svelte';
	import type { Snippet } from 'svelte';
	import { quoteMatch } from '$lib/journal/search';

	const KIND_LABEL: Record<string, string> = {
		comment: 'комментарий',
		file: 'файл',
		audio: 'звук'
	};

	let {
		author,
		time,
		entryDay,
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
		/** День, к которому запись привязана, если внесена в другой (C49): справа,
		 *  как в шапке записи в ленте. */
		entryDay?: string;
		/** Своё превью; без него — по kind из snippet и query (2.19, 3.x). */
		preview?: Snippet;
		/** Что нашлось: `post`, `comment` / `file` / `audio` (« · комментарий», « · файл», « · звук») или `day` («день»). */
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
		<div class="sr-top">
			<span>{author}<span class="sr-time">{' · '}{time}</span></span>
			{#if entryDay}<EntryDateMark label={entryDay} />{/if}
		</div>
		<div class="sub">
			{#if preview}
				{@render preview()}
			{:else if kind === 'day'}
				<span class="faint">день</span>
			{:else}
				{quoteMatch(snippet, query)}
				{#if kind === 'comment' || kind === 'file' || kind === 'audio'}
					<span class="faint">{' · '}{KIND_LABEL[kind]}</span>
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

<style>
	.sr-top {
		display: flex;
		align-items: center;
		gap: 8px;
		font-weight: 600;
	}
	.sr-time,
	.sr-top :global(.tm) {
		font-weight: 400;
		color: var(--faint);
		font-size: 11.5px;
	}
</style>
