<script lang="ts">
	import type { CircleColor } from '$lib/theme/colors';
	import { isDark } from '$lib/session/session.svelte';
	import type { Snippet } from 'svelte';

	let {
		color,
		shell = false,
		dark = false,
		wide = false,
		app = false,
		height,
		class: className = '',
		children
	}: {
		color?: CircleColor;
		shell?: boolean;
		dark?: boolean;
		wide?: boolean;
		app?: boolean;
		height?: string;
		class?: string;
		children: Snippet;
	} = $props();

	const colorClass = $derived(color && color !== 'terracotta' ? color : undefined);
	const darkClass = $derived(app ? isDark() : dark);
	const classes = $derived(
		['ph', colorClass, shell && 'shell', darkClass && 'dark', wide && 'wide', app && 'app', className]
			.filter(Boolean)
			.join(' ')
	);
</script>

<div class={classes} style:height={height}>
	{@render children()}
</div>
