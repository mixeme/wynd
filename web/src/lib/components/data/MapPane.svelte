<script lang="ts">
	import type { Snippet } from 'svelte';

	// Поле «Карты» (5.6): колонка во всю высоту под шапкой круга — плашка и
	// сама карта. Была классом экрана (план 47, хвост сверки).
	let { children, class: className = '' }: { children: Snippet; class?: string } = $props();
</script>

<div class="map-wrap {className}">
	{@render children()}
</div>
