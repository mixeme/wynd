<script lang="ts" module>
	export interface CodeLine {
		text: string;
		hi?: boolean;
		/** Строка не про эту поломку — приглушена (9.12). */
		dim?: boolean;
		cmt?: string;
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		lines,
		children,
		class: className = '',
		style = ''
	}: {
		lines?: CodeLine[];
		children?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="codeblk {className}" {style}>
	{#if lines}
		{#each lines as line, i (i)}
			{#if line.hi}<span class="hi">{line.text}</span>{:else if line.dim}<span class="dim"
					>{line.text}</span
				>{:else}{line.text}{/if}{#if line.cmt}<span class="cmt">{`  # ${line.cmt}`}</span
				>{/if}{#if i < lines.length - 1}
				{'\n'}{/if}
		{/each}
	{:else if children}
		{@render children()}
	{/if}
</div>
