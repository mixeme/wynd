<script lang="ts">
	import Button from '$ui/forms/Button.svelte';

	// Полоса «Вышла Мастерская …» над экраном (план 54, кадр 10а). «✕» прячет
	// только полосу — тихая строка внизу пульта остаётся (10а2).
	let {
		version,
		onupdate,
		onhide,
		class: className = '',
		style = ''
	}: {
		version: string;
		onupdate: () => void;
		onhide: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="ins-selfbar {className}" {style}>
	<span>Вышла Мастерская {version}</span>
	<Button variant="ghost" onclick={onupdate}>Обновить</Button>
	<button class="ins-x" type="button" aria-label="Скрыть" onclick={onhide}>✕</button>
</div>

<style>
	.ins-selfbar {
		display: flex;
		align-items: center;
		gap: 12px;
		margin: 10px 24px 0;
		padding: 6px 8px 6px 14px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--card);
	}
	.ins-selfbar span {
		flex: 1;
	}
</style>
