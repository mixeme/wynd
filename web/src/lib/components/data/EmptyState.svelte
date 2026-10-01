<script lang="ts">
	import type { Snippet } from 'svelte';
	import Mark from '$ui/Mark.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';

	// Экран или вкладка без содержимого (план 47, 2.3): заголовок по центру,
	// пояснение, действия. Отступы — по кадрам, их четыре вида:
	// screen — отдельный экран-состояние («Приглашение не действует»);
	// list — пустая улочка под шапкой (2.3); tab — пустая вкладка (3.14),
	// заголовок посередине; feed — пустая лента со знаком (3.1).
	let {
		title,
		place = 'screen',
		children,
		actions
	}: {
		title: string;
		place?: 'screen' | 'list' | 'tab' | 'feed';
		/** Пояснение под заголовком. */
		children?: Snippet;
		/** Кнопки под пояснением. */
		actions?: Snippet;
	} = $props();
</script>

<div class="empty-state {place}" class:empty={place === 'feed'}>
	{#if place === 'feed'}
		<Mark />
	{/if}
	<ScreenTitle centered>{title}</ScreenTitle>
	{#if children}
		<Hint centered>{@render children()}</Hint>
	{/if}
	{#if actions}
		<div class="empty-act">{@render actions()}</div>
	{/if}
</div>
