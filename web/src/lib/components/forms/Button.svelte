<script lang="ts">
	import type { Snippet } from 'svelte';

	type ButtonVariant = 'default' | 'colored' | 'ghost' | 'off';

	let {
		children,
		onclick,
		variant = 'default',
		disabled = false,
		loading = false,
		loadingLabel = '',
		keepFocus = false,
		stableHeight = false,
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
		/** Место рамки сохраняется при включении/выключении кнопки (локальный старт Android). */
		stableHeight?: boolean;
		class?: string;
		style?: string;
	} = $props();

	const looksOff = $derived(disabled || loading || variant === 'off');
	const variantClass = $derived(
		looksOff ? 'off' : variant === 'colored' ? 'c' : variant === 'ghost' ? 'gh' : ''
	);
	const buttonStyle = $derived(
		stableHeight
			? `${style};border-width:1px;border-style:solid${!looksOff && variant !== 'ghost' ? ';border-color:transparent' : ''}`
			: style
	);
</script>

<button
	type="button"
	class="btn {variantClass} {className}"
	style={buttonStyle}
	disabled={disabled || loading}
	aria-busy={loading || undefined}
	onmousedown={keepFocus ? (e) => e.preventDefault() : undefined}
	{onclick}
>
	{#if loading && loadingLabel}
		{loadingLabel}
	{:else}
		{@render children()}
	{/if}
</button>
