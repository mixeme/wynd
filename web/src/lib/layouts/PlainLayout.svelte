<script lang="ts">
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
		children
	}: {
		color?: CircleColor;
		shell?: boolean;
		dark?: boolean;
		app?: boolean;
		height?: string;
		class?: string;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame {color} {shell} {dark} {app} {height} class={className}>
	{#if !app}
		<StatusBar />
	{/if}
	{@render children()}
</PhoneFrame>
