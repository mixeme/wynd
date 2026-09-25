<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import MonthLabel from '$ui/data/MonthLabel.svelte';
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
		goto('/circles');
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
	active="Сетка"
	commentBar={false}
	onback={goBack}
>
	{#if loading}
		<Hint style="margin:24px 16px">Загрузка…</Hint>
	{:else if error && !tiles.length}
		<Hint style="margin:24px 16px">{error}</Hint>
	{:else}
		{#if tiles.length}
			<Hint style="margin-top:12px">
				{pluralPosts(tiles.length)} с фотографиями из {pluralPosts(totalPosts)}
			</Hint>
		{/if}
		{#each monthGroups as group (group.month)}
			<MonthLabel>{group.label}</MonthLabel>
			<PhotoGrid>
				{#each group.items as tile (tile.postId)}
					<button type="button" class="pic" onclick={() => openPost(tile.postId)}>
						{#if mediaUrls[tile.blobId]}
							<img src={mediaUrls[tile.blobId]} alt="" />
						{/if}
						{#if tile.photoCount > 1}
							<span class="cnt" style="bottom:6px;right:6px;padding:3px 8px">{tile.photoCount}</span>
						{/if}
					</button>
				{/each}
			</PhotoGrid>
		{/each}
		{#if !tiles.length}
			<Hint style="margin:24px 16px">Записей с фотографиями пока нет</Hint>
		{/if}
	{/if}
</CircleLayout>

<style>
	.pic {
		aspect-ratio: 1 / 1;
		border-radius: 2px;
		overflow: hidden;
		background: var(--tint);
		position: relative;
		border: none;
		padding: 0;
		cursor: pointer;
	}
	.pic img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
</style>
