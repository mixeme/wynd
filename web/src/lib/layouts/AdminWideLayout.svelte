<script lang="ts">
	import type { AdminNavItem } from '$ui/admin/AdminNav.svelte';
	import AdminBar from '$ui/chrome/AdminBar.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import type { Snippet } from 'svelte';

	let {
		active = 'Хранилище',
		server,
		app = false,
		nav = true,
		height,
		cols = false,
		class: className = '',
		children
	}: {
		active?: AdminNavItem;
		server?: string;
		app?: boolean;
		nav?: boolean;
		height?: string;
		cols?: boolean;
		class?: string;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame shell wide {app} {height} class={className}>
	<AdminBar {active} {server} {nav} links={app} />
	{#if cols}
		<div class="cols">{@render children()}</div>
	{:else}
		{@render children()}
	{/if}
</PhoneFrame>
