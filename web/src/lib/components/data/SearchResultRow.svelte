<script lang="ts">
	import Row from '$ui/data/Row.svelte';
	import type { Snippet } from 'svelte';

	let {
		author,
		time,
		preview,
		thumb,
		thumbUrl,
		thumbVariant,
		onclick,
		class: className = '',
		style = ''
	}: {
		author: string;
		time: string;
		preview: Snippet;
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
			{@render preview()}
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
