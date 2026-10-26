<script lang="ts">
	import type { Snippet } from 'svelte';

	// Ряд кнопок внизу экрана Мастерской (план 54). Неброское действие справа —
	// класс ins-push на нём (окно 5а).
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

<div class="ins-foot {className}" {style}>
	{@render children()}
</div>

<style>
	.ins-foot {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 12px;
		margin-top: 18px;
	}
	.ins-foot :global(.ins-push) {
		margin-left: auto;
	}
</style>
