<script lang="ts">
	import AppBar from '$ui/chrome/AppBar.svelte';
	import ConnectionStrip from '$ui/chrome/ConnectionStrip.svelte';
	import { connectionNotice } from '$lib/session/connection.svelte';
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

	// Полоса прокрутки улочки: Chrome и Edge рисуют её и тогда, когда строки
	// помещаются целиком (заметно в узкой левой колонке на широком окне).
	// Прячем, пока прокручивать нечего; при переполнении она возвращается.
	let bodyEl = $state<HTMLDivElement>();
	// До первого замера считаем, что помещается: так полоса не мигает на входе.
	let bodyFits = $state(true);

	$effect(() => {
		const el = bodyEl;
		if (!el) return;
		// +1 px — округление субпикселей: полоса из-за него не нужна.
		const update = () => {
			bodyFits = el.scrollHeight <= el.clientHeight + 1;
		};
		update();
		const ro = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(update);
		ro?.observe(el);
		if (el.firstElementChild) ro?.observe(el.firstElementChild);
		const mo = new MutationObserver(update);
		mo.observe(el, { childList: true, subtree: true, characterData: true });
		return () => {
			ro?.disconnect();
			mo.disconnect();
		};
	});
</script>

<PhoneFrame shell {dark} {app} {height} class={className}>
	{#if !app}
		<StatusBar />
	{/if}
	<AppBar {onsearch} {searchDisabled} {onsettings} />
	{#if app}
		<ConnectionStrip notice={connectionNotice()} />
		<div class="shell-body" class:fit={bodyFits} bind:this={bodyEl}>
			{@render children()}
		</div>
	{:else}
		{@render children()}
	{/if}
	{#if fab}
		<Fab menuOpen={fabMenuOpen} items={fabMenuItems} onclose={onfabmenuclose}>{@render fab()}</Fab>
	{/if}
</PhoneFrame>
