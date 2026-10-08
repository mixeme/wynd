<script lang="ts">
	import type { Snippet } from 'svelte';

	type ButtonVariant = 'default' | 'colored' | 'ghost' | 'off';

	let {
		children,
		onclick,
		variant = 'default',
		disabled = false,
		loading = false,
		keepFocus = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		onclick: () => void;
		variant?: ButtonVariant;
		disabled?: boolean;
		loading?: boolean;
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
	onmousedown={keepFocus ? (e) => e.preventDefault() : undefined}
	{onclick}
>
	{#if loading}
		…
	{:else}
		{@render children()}
	{/if}
</button>
