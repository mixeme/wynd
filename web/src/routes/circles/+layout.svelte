<script lang="ts">
	import { page } from '$app/state';
	import CircleSettingsNav from '$lib/layouts/CircleSettingsNav.svelte';
	import CirclesSplitLayout from '$lib/layouts/CirclesSplitLayout.svelte';
	import PayGateLayout from '$lib/layouts/PayGateLayout.svelte';
	import SearchPane from '$lib/layouts/SearchPane.svelte';
	import StreetLayout from '$lib/layouts/StreetLayout.svelte';
	import { isCircleSettingsPath } from '$lib/navigation/wideDetail';
	import { wideSource } from '$lib/navigation/wideSource.svelte';
	import { splitLayout } from '$lib/session/layout.svelte';
	import { untrack, type Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	const path = $derived(page.url.pathname);
	const id = $derived(page.params.id ?? '');
	// Улочка без выбора и сам круг — две колонки. Создание круга и вступление
	// по ссылке остаются одноколоночными: это переходы, а не чтение круга.
	const street = $derived(path === '/circles');
	const circle = $derived(Boolean(id) && !path.endsWith('/join'));
	const split = $derived(splitLayout() && (street || circle));
	// Настройки круга и их разделы получают слева пункты настроек, остальные
	// экраны круга — улицу.
	const settings = $derived(Boolean(id) && isCircleSettingsPath(path, id));
	// Круг открыт из поиска: слева остаются результаты, а не улица. Источник
	// читаем при смене круга, а не на каждую правку запроса: иначе левая
	// колонка пересоздавала бы правую и поиск с кругом не догружались.
	const searchSource = $derived.by(() => {
		void id;
		return untrack(() => wideSource());
	});
</script>

<PayGateLayout>
	{#if split}
		<CirclesSplitLayout empty={street}>
			{#snippet sidebar()}
				{#if settings}
					<CircleSettingsNav />
				{:else if circle && searchSource}
					<SearchPane sourceUrl={searchSource.url} />
				{:else}
					<StreetLayout selectedId={circle ? id : null} hideSearch />
				{/if}
			{/snippet}
			{@render children()}
		</CirclesSplitLayout>
	{:else}
		{@render children()}
	{/if}
</PayGateLayout>
