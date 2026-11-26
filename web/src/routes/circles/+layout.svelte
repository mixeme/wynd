<script lang="ts">
	import { page } from '$app/state';
	import CirclesSplitLayout from '$lib/layouts/CirclesSplitLayout.svelte';
	import PayGateLayout from '$lib/layouts/PayGateLayout.svelte';
	import StreetLayout from '$lib/layouts/StreetLayout.svelte';
	import { railLayout, splitLayout } from '$lib/session/layout.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	const path = $derived(page.url.pathname);
	const id = $derived(page.params.id ?? '');
	// Улочка без выбора и сам круг — две колонки. Создание круга и вступление
	// по ссылке остаются одноколоночными: это переходы, а не чтение круга.
	const street = $derived(path === '/circles');
	const circle = $derived(Boolean(id) && !path.endsWith('/join'));
	const split = $derived(splitLayout() && (street || circle));
	const rail = $derived(railLayout());
</script>

<PayGateLayout>
	{#if split}
		<CirclesSplitLayout empty={street} {rail}>
			{#snippet sidebar()}
				<StreetLayout selectedId={circle ? id : null} {rail} hideSearch />
			{/snippet}
			{@render children()}
		</CirclesSplitLayout>
	{:else}
		{@render children()}
	{/if}
</PayGateLayout>
