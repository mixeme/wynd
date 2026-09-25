<script lang="ts">
	import type { Snippet } from 'svelte';

	type FabMenuItem = { label: string; onclick: () => void };

	let {
		children,
		dark = false,
		menuOpen = false,
		items = [],
		onclose,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		dark?: boolean;
		menuOpen?: boolean;
		items?: FabMenuItem[];
		/** Escape в открытом меню (UI-3). */
		onclose?: () => void;
		class?: string;
		style?: string;
	} = $props();

	let menuEl = $state<HTMLDivElement | null>(null);

	function menuItems(): HTMLButtonElement[] {
		return [...(menuEl?.querySelectorAll<HTMLButtonElement>('.fab-menu-item') ?? [])];
	}

	// Открытое меню забирает фокус на первый пункт: читалка слышит меню, а не
	// кнопку под ним (план 42, UI-3).
	$effect(() => {
		if (menuOpen && menuEl) menuItems()[0]?.focus();
	});

	function onKeydown(e: KeyboardEvent) {
		if (!menuOpen) return;
		if (e.key === 'Escape') {
			e.preventDefault();
			onclose?.();
			return;
		}
		if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
		const list = menuItems();
		if (!list.length) return;
		e.preventDefault();
		const idx = list.indexOf(document.activeElement as HTMLButtonElement);
		const step = e.key === 'ArrowDown' ? 1 : -1;
		list[(idx + step + list.length) % list.length].focus();
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="fab-wrap {className}" {style}>
	{#if menuOpen && items.length}
		<div class="fab-menu" role="menu" bind:this={menuEl}>
			{#each items as item (item.label)}
				<button type="button" class="fab-menu-item" role="menuitem" onclick={() => item.onclick()}>
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
