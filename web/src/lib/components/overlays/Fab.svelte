<script lang="ts">
	import type { Snippet } from 'svelte';

	type FabMenuItem = { label: string; onclick: () => void };

	let {
		children,
		dark = false,
		menuOpen = false,
		items = [],
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		dark?: boolean;
		menuOpen?: boolean;
		items?: FabMenuItem[];
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="fab-wrap {className}" {style}>
	{#if menuOpen && items.length}
		<div class="fab-menu">
			{#each items as item (item.label)}
				<button type="button" class="fab-menu-item" onclick={() => item.onclick()}>
					{item.label}
				</button>
			{/each}
		</div>
	{/if}
	<div
		class="fab"
		style:background={dark ? '#EFE9E0' : undefined}
		style:color={dark ? '#211E1C' : undefined}
	>
		{@render children()}
	</div>
</div>
