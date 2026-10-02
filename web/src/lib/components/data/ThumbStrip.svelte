<script lang="ts">
	import type { Snippet } from 'svelte';

	// Ряд выбранных снимков и вложений на новой записи (4.2) с плюсом в
	// конце. Был классом экрана (план 47, хвост сверки).
	let { children, class: className = '' }: { children: Snippet; class?: string } = $props();
</script>

<div class="thumbs {className}">
	{@render children()}
</div>
