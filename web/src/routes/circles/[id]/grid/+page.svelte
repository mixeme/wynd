<script lang="ts">
	import { goUp } from '$lib/navigation/up';
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
	import { pluralPosts } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadDays } from '$lib/journal/days';
	import { groupGridTiles, loadGrid, type GridTile } from '$lib/journal/grid';
	import { groupByMonth } from '$lib/journal/group';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let tiles = $state<GridTile[]>([]);
	let totalPosts = $state(0);
	let loading = $state(true);
	let error = $state('');
	let mediaUrls = $state<Record<string, string>>({});

	async function resolveUrls(items: GridTile[]) {
		const next: Record<string, string> = { ...mediaUrls };
		for (const tile of items) {
			if (!next[tile.blobId]) {
				try {
					next[tile.blobId] = await getMediaUrl(circle.origin, tile.blobId);
				} catch {
					/* skip */
				}
			}
		}
		mediaUrls = next;
	}

	async function loadData() {
		error = '';
		try {
			const [gridSnap, daysSnap] = await Promise.all([
				loadGrid(circle.origin, circle.circleId),
				loadDays(circle.origin, circle.circleId)
			]);
			totalPosts = daysSnap.days.reduce((sum, d) => sum + d.post_count, 0);
			tiles = groupGridTiles(gridSnap.items).map((tile) => ({
				...tile,
				entryDate: tile.entryDate
			}));
			await resolveUrls(tiles);
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить сетку';
		} finally {
			loading = false;
		}
	}

	const monthGroups = $derived(groupByMonth(tiles));

	// Обновление жестом (3.5), как в ленте: тянешь экран от верха.
	let listEl: HTMLDivElement | undefined = $state();
	const ptr = new PullRefresh(() => listEl?.scrollTop ?? 0, () => loadData());
	onDestroy(() => ptr.destroy());

	onMount(() => {
		void loadData();
		const unsub = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['grid', 'days'],
			refetch: loadData
		});
		return unsub;
	});

	function goBack() {
		goUp('/circles');
	}

	function openPost(postId: string) {
		goto(`/circles/${circle.circleId}/posts/${postId}/album`);
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
		ontouchstart={ptr.start}
		ontouchmove={ptr.move}
		ontouchend={ptr.end}
	>
	{#if loading}
		<Loading />
	{:else if error && !tiles.length}
		<Hint class="gutter-24">{error}</Hint>
	{:else}
		{#if tiles.length}
			<Hint class="mt-12">
				{pluralPosts(tiles.length)} с фотографиями из {pluralPosts(totalPosts)}
			</Hint>
		{/if}
		{#each monthGroups as group (group.month)}
			<MonthLabel>{group.label}</MonthLabel>
			<PhotoGrid>
				{#each group.items as tile (tile.postId)}
					<MediaTile
						variant="grid"
						src={mediaUrls[tile.blobId]}
						count={tile.photoCount}
						onclick={() => openPost(tile.postId)}
					/>
				{/each}
			</PhotoGrid>
		{/each}
		{#if !tiles.length}
			<Hint class="gutter-24">Записей с фотографиями пока нет</Hint>
		{/if}
	{/if}
	</div>
</CircleLayout>

