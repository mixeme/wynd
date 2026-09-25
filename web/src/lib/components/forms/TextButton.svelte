<script lang="ts">
	import type { Snippet } from 'svelte';

	type TextButtonVariant = 'link' | 'admin' | 'adminBox' | 'bar' | 'barAction';

	let {
		children,
		onclick,
		variant = 'link',
		active = false,
		disabled = false,
		loading = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		onclick: () => void;
		variant?: TextButtonVariant;
		active?: boolean;
		disabled?: boolean;
		loading?: boolean;
		class?: string;
		style?: string;
	} = $props();

	const variantClass = $derived(
		variant === 'admin'
			? 'act'
			: variant === 'adminBox'
				? 'inp'
				: variant === 'bar'
					? 't'
					: variant === 'barAction'
						? `rt${active ? ' on' : ''}`
						: 'under'
	);
</script>

<button
	type="button"
	class="{variantClass} {className}"
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
