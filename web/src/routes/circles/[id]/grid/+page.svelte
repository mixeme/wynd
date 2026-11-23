<script lang="ts">
	import { albumHref } from '$lib/journal/present';
	import { navigateBack } from '$lib/navigation/up';
	import PullRefreshBand from '$ui/data/PullRefresh.svelte';
	import { PullRefresh } from '$lib/gestures/pullRefresh.svelte';
	import { goto } from '$app/navigation';
	import { getContext, onDestroy, onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import MonthLabel from '$ui/data/MonthLabel.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { isAccessError } from '$lib/api/client';
	import { gridSummary } from '$lib/journal/summary-badge';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadDays, loadDaysCached } from '$lib/journal/days';
	import { groupGridTiles, loadGrid, loadGridCached, type GridTile } from '$lib/journal/grid';
	import { paintFromSnapshots } from '$lib/journal/snapshot-first';
	import { fetchOlderPage, mergePages, refetchOlder } from '$lib/journal/pages';
	import type { GridItem } from '$lib/journal/types';
	import { groupByMonth } from '$lib/journal/group';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let tiles = $state<GridTile[]>([]);
	// Порции «Сетки» (C18): первая из снимка, старшие — при прокрутке к концу.
	let firstItems: GridItem[] = [];
	let olderItems = $state<GridItem[]>([]);
	let nextBefore = $state<string | undefined>(undefined);
	let loadingOlder = $state(false);
	const itemKey = (i: GridItem) => `${i.post_id}:${i.blob_id}`;
	let totalPosts = $state(0);
	let loading = $state(true);
	let error = $state('');
	let mediaUrls = $state<Record<string, string>>({});
	// Свежий ответ сети уже на экране — поздний снимок его не перетирает (C100).
	let freshApplied = false;
	let cachedRequested = false;

	async function resolveUrls(items: GridTile[]) {
		const next: Record<string, string> = { ...mediaUrls };
		for (const tile of items) {
			if (next[tile.tileId]) continue;
			try {
				next[tile.tileId] = await getMediaUrl(circle.origin, tile.tileId);
			} catch {
				/* skip */
			}
		}
		mediaUrls = next;
	}

	// Снимки сетки и сводки дней — первый кадр вкладки (C100-1): «Сетка» не
	// ждёт сети, свежесть приходит ответом следом.
	async function loadCached() {
		if (cachedRequested) return;
		cachedRequested = true;
		const painted = await paintFromSnapshots(
			() => freshApplied,
			async () => {
				const snap = await loadGridCached(circle.origin, circle.circleId);
				if (!snap) return undefined;
				return () => {
					firstItems = snap.items;
					nextBefore = snap.next_before;
				};
			},
			async () => {
				const snap = await loadDaysCached(circle.origin, circle.circleId);
				if (!snap) return undefined;
				return () => {
					totalPosts = snap.days.reduce((sum, d) => sum + d.post_count, 0);
				};
			}
		);
		if (!painted) return;
		setTiles();
		loading = false;
		void resolveUrls(tiles);
	}

	async function loadData() {
		error = '';
		try {
			const [gridSnap, daysSnap] = await Promise.all([
				loadGrid(circle.origin, circle.circleId),
				loadDays(circle.origin, circle.circleId)
			]);
			freshApplied = true;
			totalPosts = daysSnap.days.reduce((sum, d) => sum + d.post_count, 0);
			firstItems = gridSnap.items;
			nextBefore = gridSnap.next_before;
			if (olderItems.length) {
				// Граница первой порции сдвинулась — перечитываем догруженное от неё.
				const again = await refetchOlder(
					loadOlderGrid,
					gridSnap.next_before,
					olderItems.length
				).catch(() => ({ items: olderItems, next: nextBefore }));
				olderItems = again.items;
				nextBefore = again.next;
			}
			setTiles();
			await resolveUrls(tiles);
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить сетку';
		} finally {
			loading = false;
		}
	}

	function setTiles() {
		tiles = groupGridTiles(mergePages(firstItems, olderItems, itemKey));
	}

	async function loadOlderGrid(before: string) {
		const page = await fetchOlderPage<{ items: GridItem[] }>(
			circle.origin,
			`/circles/${circle.circleId}/grid`,
			before
		);
		return { items: page.items, next: page.next_before };
	}

	async function loadOlder() {
		if (!nextBefore || loadingOlder) return;
		loadingOlder = true;
		try {
			const page = await loadOlderGrid(nextBefore);
			olderItems = [...olderItems, ...page.items];
			nextBefore = page.next;
			setTiles();
			void resolveUrls(tiles);
		} catch {
			// Без сети старшее не догрузится.
		} finally {
			loadingOlder = false;
		}
	}

	function onListScroll() {
		const el = listEl;
		if (!el || !nextBefore) return;
		if (el.scrollTop + el.clientHeight * 2.5 >= el.scrollHeight) void loadOlder();
	}

	const monthGroups = $derived(groupByMonth(tiles));

	// Обновление жестом (3.5), как в ленте: тянешь экран от верха.
	let listEl: HTMLDivElement | undefined = $state();
	const ptr = new PullRefresh(
		() => listEl?.scrollTop ?? 0,
		() => loadData()
	);
	onDestroy(() => ptr.destroy());

	onMount(() => {
		void (async () => {
			await loadCached();
			await loadData();
		})();
		const unsub = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['grid', 'days'],
			refetch: loadData
		});
		return unsub;
	});

	function goBack() {
		void navigateBack();
	}

	function openPost(postId: string) {
		const tile = tiles.find((t) => t.postId === postId);
		goto(albumHref(circle.circleId, postId, tile?.photoCount ?? 0));
	}
</script>

<CircleLayout
	app
	color={circle.color}
	title={circle.name}
	identity={circle.identityName}
	avatar={circle.identityInitial}
	avatarSrc={circle.avatarUrl}
	circleId={circle.circleId}
	identitySettingsLink={circle.canWrite}
	active="Сетка"
	commentBar={false}
	onback={goBack}
>
	<PullRefreshBand pull={ptr.state} />
	<div
		class="feed"
		role="feed"
		aria-label="Сетка"
		bind:this={listEl}
		onscroll={onListScroll}
		ontouchstart={ptr.start}
		ontouchmove={ptr.move}
		ontouchend={ptr.end}
	>
		{#if loading}
			<Loading />
		{:else if error && !tiles.length}
			<Hint class="gutter">{error}</Hint>
		{:else}
			{#if tiles.length}
				<Hint class="mt-8">
					{gridSummary(tiles.length, totalPosts)}
				</Hint>
			{/if}
			{#each monthGroups as group (group.month)}
				<MonthLabel>{group.label}</MonthLabel>
				<PhotoGrid>
					{#each group.items as tile (tile.postId)}
						<MediaTile
							variant="grid"
							src={mediaUrls[tile.tileId]}
							kind={tile.kind}
							durationMs={tile.durationMs}
							count={tile.photoCount}
							onclick={() => openPost(tile.postId)}
						/>
					{/each}
				</PhotoGrid>
			{/each}
			{#if loadingOlder}
				<Hint class="gutter" centered>Загружаем записи постарше…</Hint>
			{/if}
			{#if !tiles.length}
				<Hint class="gutter">Записей с фото и видео пока нет</Hint>
			{/if}
		{/if}
	</div>
</CircleLayout>
