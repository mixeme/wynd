<script lang="ts">
	import CirclesSplitLayout from '$lib/layouts/CirclesSplitLayout.svelte';
	import PayGateLayout from '$lib/layouts/PayGateLayout.svelte';
	import SearchPane from '$lib/layouts/SearchPane.svelte';
	import { wideSource } from '$lib/navigation/wideSource.svelte';
	import { splitLayout } from '$lib/session/layout.svelte';
	import { untrack, type Snippet } from 'svelte';

	// План 76: на широком окне поиск — левая колонка, справа открывается
	// найденный круг. Ворота оплаты остаются снаружи, во весь экран.
	let { children }: { children: Snippet } = $props();

	const split = $derived(splitLayout());
	// Источник читаем один раз при входе в поиск: пока колонка открыта, её
	// запрос живёт своей жизнью и не должен пересоздавать каркас.
	const sourceUrl = untrack(() => wideSource()?.url);
</script>

<PayGateLayout>
	{#if split}
		<CirclesSplitLayout
			empty
			emptyTitle="Выберите запись"
			emptyText="Откройте результат в списке слева — он появится здесь."
		>
			{#snippet sidebar()}
				<SearchPane {sourceUrl} />
			{/snippet}
			{@render children()}
		</CirclesSplitLayout>
	{:else}
		{@render children()}
	{/if}
</PayGateLayout>
