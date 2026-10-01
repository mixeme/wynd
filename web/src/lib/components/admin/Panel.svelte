<script lang="ts">
	import type { Snippet } from 'svelte';

	// Карточка админки `.panel` (план 47, 2.20): рамка, фон карточки, скругление.
	// Была собрана руками в четырёх экранах, в мастере — через строку классов.
	let {
		children,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="panel {className}" {style}>
	{@render children()}
</div>
