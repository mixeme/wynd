<script lang="ts">
	import type { Snippet } from 'svelte';

	// Нижняя полоса новой записи (4.2; план 47, 3.7): кнопки вложений слева,
	// мелкая подпись справа («до 32 КБ · как Мышь»).
	let { tools, note }: { tools: Snippet; note?: Snippet } = $props();
</script>

<div class="compose-bar">
	<div class="tools">
		{@render tools()}
		{#if note}
			<span class="who">{@render note()}</span>
		{/if}
	</div>
</div>
