<script lang="ts">
	import type { Snippet } from 'svelte';

	// Список строк MarkRow (план 54): последняя — без черты снизу.
	// continues — сразу ниже идёт второй список того же дела (проверка, потом
	// подпись): стык читается обычной строкой, поэтому шаг между списками снят,
	// а черта под последней строкой остаётся — иначе на стыке пропадает
	// разделитель.
	let {
		children,
		class: className = '',
		style = '',
		continues = false
	}: {
		children: Snippet;
		class?: string;
		style?: string;
		continues?: boolean;
	} = $props();
</script>

<div class="ins-rows {className}" class:ins-continues={continues} {style}>
	{@render children()}
</div>

<style>
	.ins-rows {
		margin-bottom: var(--sp-2);
	}
	.ins-rows.ins-continues {
		margin-bottom: 0;
	}
	.ins-rows.ins-continues > :global(.ins-row:last-child) {
		border-bottom: 1px solid var(--line);
	}
</style>
