<script lang="ts">
	import type { Snippet } from 'svelte';

	// Список строк MarkRow (план 54): последняя — без черты снизу.
	// Два списка шагов подряд (проверка, потом подпись) — один ход работы, а не
	// два блока: у первого шаг до следующего снимается, а черта под последней
	// строкой остаётся — иначе на стыке пропадает разделитель и зазор выходит
	// больше строки. Решает сам компонент по соседнему .ins-steps, а не вызов.
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

<div class="ins-rows {className}" {style}>
	{@render children()}
</div>

<style>
	.ins-rows {
		margin-bottom: var(--sp-2);
	}
	/* :global — класс ins-steps ставит StepList, в этом шаблоне его нет;
	   без :global Svelte выбросил бы правило как неиспользуемое. */
	:global(.ins-rows.ins-steps:has(+ .ins-rows.ins-steps)) {
		margin-bottom: 0;
	}
	:global(.ins-rows.ins-steps:has(+ .ins-rows.ins-steps) > .ins-row:last-child) {
		border-bottom: 1px solid var(--line);
	}
</style>
