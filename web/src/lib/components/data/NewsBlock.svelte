<script lang="ts">
	import SectionLabel from '$ui/data/SectionLabel.svelte';

	// Одна версия на странице «Что нового» (7.11): подпись и строки изменений.
	let {
		version,
		day,
		items
	}: {
		version: string;
		/** День выхода словами: «8 октября». */
		day: string;
		items: string[];
	} = $props();
</script>

<SectionLabel raw>
	<span class="ink">Wynd {version}</span>{#if day}{' · '}{day}{/if}
</SectionLabel>
<ul class="news">
	{#each items as item (item)}
		<li>{item}</li>
	{/each}
</ul>
