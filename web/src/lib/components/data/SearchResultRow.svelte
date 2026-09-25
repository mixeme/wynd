<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		author,
		time,
		preview,
		thumb,
		thumbVariant,
		onclick,
		class: className = '',
		style = ''
	}: {
		author: string;
		time: string;
		preview: Snippet;
		thumb?: boolean;
		thumbVariant?: string;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

{#if onclick}
	<button type="button" class="row2 {className}" {style} {onclick}>
		<div class="g">
			<div style="font-weight:600">
				{author}<span style="font-weight:400;color:var(--faint);font-size:11.5px"> · {time}</span>
			</div>
			<div class="sub">
				{@render preview()}
			</div>
		</div>
		{#if thumb}
			<div class="thumb pic {thumbVariant ?? 'p1'}"></div>
		{/if}
	</button>
{:else}
	<div class="row2 {className}" {style}>
		<div class="g">
			<div style="font-weight:600">
				{author}<span style="font-weight:400;color:var(--faint);font-size:11.5px"> · {time}</span>
			</div>
			<div class="sub">
				{@render preview()}
			</div>
		</div>
		{#if thumb}
			<div class="thumb pic {thumbVariant ?? 'p1'}"></div>
		{/if}
	</div>
{/if}
