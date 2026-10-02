<script lang="ts">
	import type { Snippet } from 'svelte';

	// Нить комментариев под записью (4.5): черта над первым и запас снизу под
	// полосу комментария. Была классом экрана записи (план 47, хвост сверки).
	let { children, class: className = '' }: { children: Snippet; class?: string } = $props();
</script>

<div class="thread {className}">
	{@render children()}
</div>
