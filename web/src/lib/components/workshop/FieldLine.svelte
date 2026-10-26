<script lang="ts">
	import type { Snippet } from 'svelte';

	// Поле и кнопка в ряд (план 54): папка бэкапа, файл архива — Input с
	// классом ins-grow и «Выбрать».
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

<div class="ins-copyline {className}" {style}>
	{@render children()}
</div>

<style>
	.ins-copyline {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 8px 0;
	}
</style>
