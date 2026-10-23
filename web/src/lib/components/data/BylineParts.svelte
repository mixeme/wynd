<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import type { BylinePart } from '$lib/journal/present';

	// «Аня · 20 октября, 17:37 · [календарь] 28 сентября» одной строкой —
	// шапка альбома и подпись лайтбокса (C49). Значок — тот же, что у дня
	// записи в ленте и на экране записи.
	let { parts }: { parts: BylinePart[] } = $props();
</script>

{#each parts as part, i (i)}{#if i}{' · '}{/if}{#if part.icon}<span
			style:display="inline-flex"
			style:align-items="center"
			style:gap="4px"
			style:vertical-align="bottom"><Icon name={part.icon} size="xs" />{part.text}</span
		>{:else}{part.text}{/if}{/each}
