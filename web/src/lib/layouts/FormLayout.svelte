<script lang="ts">
	import BackBar from '$ui/chrome/BackBar.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import type { CircleColor } from '$lib/theme/colors';
	import type { Snippet } from 'svelte';

	let {
		color,
		shell = false,
		dark = false,
		app = false,
		height,
		class: className = '',
		title,
		right,
		search,
		compact = false,
		bar,
		onback,
		children
	}: {
		color?: CircleColor;
		shell?: boolean;
		dark?: boolean;
		app?: boolean;
		height?: string;
		class?: string;
		title?: string;
		right?: string;
		search?: string;
		compact?: boolean;
		bar?: Snippet;
		onback?: () => void;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame {color} {shell} {dark} {app} {height} class={className}>
	{#if !app}
		<StatusBar />
	{/if}
	{#if bar}
		<BackBar {compact} {onback}>{@render bar()}</BackBar>
	{:else}
		<BackBar {title} {right} {search} {compact} {onback} />
	{/if}
	{@render children()}
</PhoneFrame>
