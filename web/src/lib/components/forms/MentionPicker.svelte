<script lang="ts">
	import type { Snippet } from 'svelte';

	// Список упоминаний (4.3). floating — поверх экрана, под полем текста:
	// снимки и дата под ним не сдвигаются (владелец, 2026-10-15); hint —
	// подсказка последней строкой в самом списке.
	let {
		children,
		floating = false,
		hint
	}: { children: Snippet; floating?: boolean; hint?: string } = $props();
</script>

{#if floating}
	<div class="men-anchor">
		<div class="men-float">
			<div class="men-pick">
				{@render children()}
				{#if hint}
					<div class="men-hint">{hint}</div>
				{/if}
			</div>
		</div>
	</div>
{:else}
	<div class="men-pick">
		{@render children()}
		{#if hint}
			<div class="men-hint">{hint}</div>
		{/if}
	</div>
{/if}
