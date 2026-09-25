<script lang="ts">
	import AppBar from '$ui/chrome/AppBar.svelte';
	import Fab from '$ui/overlays/Fab.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import type { Snippet } from 'svelte';

	let {
		dark = false,
		app = false,
		height,
		class: className = '',
		fab,
		fabMenuOpen = false,
		fabMenuItems = [],
		onfabmenuclose,
		onsearch,
		searchDisabled = false,
		onsettings,
		children
	}: {
		dark?: boolean;
		app?: boolean;
		height?: string;
		class?: string;
		fab?: Snippet;
		fabMenuOpen?: boolean;
		fabMenuItems?: { label: string; onclick: () => void }[];
		onfabmenuclose?: () => void;
		onsearch?: () => void;
		searchDisabled?: boolean;
		onsettings?: () => void;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame shell {dark} {app} {height} class={className}>
	{#if !app}
		<StatusBar />
	{/if}
	<AppBar {onsearch} {searchDisabled} {onsettings} />
	{#if app}
		<div class="shell-body">
			{@render children()}
		</div>
	{:else}
		{@render children()}
	{/if}
	{#if fab}
		<Fab menuOpen={fabMenuOpen} items={fabMenuItems} onclose={onfabmenuclose}>{@render fab()}</Fab>
	{/if}
</PhoneFrame>
