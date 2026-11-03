<script lang="ts">
	import type { Snippet } from 'svelte';

	type ButtonVariant = 'default' | 'colored' | 'ghost' | 'off';

	let {
		children,
		onclick,
		variant = 'default',
		disabled = false,
		loading = false,
		loadingLabel = '…',
		keepFocus = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		onclick: () => void;
		variant?: ButtonVariant;
		disabled?: boolean;
		loading?: boolean;
		/** Текст внутри кнопки на время выполнения действия. */
		loadingLabel?: string;
		/**
		 * Кнопка рядом с полем, которое она сохраняет: нажатие не уводит фокус
		 * из поля. Иначе телефон прячет клавиатуру уже на нажатии, до ответа
		 * сервера, и в Firefox она успевала мигнуть; при ошибке — пропадала,
		 * хотя править ещё надо. Клавиатура уходит вместе с полем.
		 */
		keepFocus?: boolean;
		class?: string;
		style?: string;
	} = $props();

	const looksOff = $derived(disabled || loading || variant === 'off');
	const variantClass = $derived(
		looksOff ? 'off' : variant === 'colored' ? 'c' : variant === 'ghost' ? 'gh' : ''
	);
</script>

<button
	type="button"
	class="btn {variantClass} {className}"
	{style}
	disabled={disabled || loading}
	aria-busy={loading || undefined}
	onmousedown={keepFocus ? (e) => e.preventDefault() : undefined}
	{onclick}
>
	{#if loading}
		{loadingLabel}
	{:else}
		{@render children()}
	{/if}
</button>
