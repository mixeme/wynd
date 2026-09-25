<script lang="ts">
	import type { Snippet } from 'svelte';

	type ButtonVariant = 'default' | 'colored' | 'ghost' | 'off';

	let {
		children,
		onclick,
		variant = 'default',
		disabled = false,
		loading = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		onclick: () => void;
		variant?: ButtonVariant;
		disabled?: boolean;
		loading?: boolean;
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
	{onclick}
>
	{#if loading}
		…
	{:else}
		{@render children()}
	{/if}
</button>
