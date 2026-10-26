<script lang="ts">
	import type { Snippet } from 'svelte';

	// Строка окна Мастерской (план 54): значок — текст — действие. Значок стоит
	// вровень с первой строкой текста; без action текст не растягивается.
	let {
		mark,
		tone,
		dim = false,
		action,
		children,
		class: className = '',
		style = ''
	}: {
		/** ✓, !, …, ○, 1, ↓, +, ~, · */
		mark: string | number;
		tone?: 'ok' | 'note' | 'block';
		/** Ещё не начатый шаг — строка бледная. */
		dim?: boolean;
		action?: Snippet;
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="ins-row {className}" class:ins-dim={dim} {style}>
	<span class="ins-mark {tone ?? ''}">{mark}</span>
	<div class:ins-grow={Boolean(action)}>{@render children()}</div>
	{@render action?.()}
</div>

<style>
	.ins-row {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		padding: 7px 0;
		border-bottom: 1px solid var(--line);
	}
	.ins-row:last-child {
		border-bottom: 0;
	}
	.ins-row.ins-dim {
		color: var(--faint);
	}
</style>
