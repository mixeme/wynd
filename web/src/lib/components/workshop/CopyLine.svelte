<script lang="ts">
	import Button from '$ui/forms/Button.svelte';

	// Строка, которую человек вставит сам (команда на сервере, адрес для
	// записи A), и «Скопировать» рядом (план 54).
	let {
		text,
		copied = false,
		oncopy,
		class: className = '',
		style = ''
	}: {
		text: string;
		/** Только что скопировали — на кнопке «Скопировано». */
		copied?: boolean;
		oncopy: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="ins-copyline {className}" {style}>
	<div class="ins-mono">{text}</div>
	<Button variant="ghost" onclick={oncopy}>{copied ? 'Скопировано' : 'Скопировать'}</Button>
</div>

<style>
	.ins-copyline {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 8px 0;
	}
	.ins-mono {
		flex: 1;
		min-width: 0;
		padding: 8px 11px;
		border: 1px solid var(--line);
		border-radius: 8px;
		background: var(--paper);
		user-select: all;
	}
</style>
