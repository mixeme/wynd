<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import PullRefreshBand from '$ui/data/PullRefresh.svelte';
	import { PullRefresh } from '$lib/gestures/pullRefresh.svelte';
	import { goto } from '$app/navigation';
	import { getContext, onDestroy, onMount } from 'svelte';
	import DayCard from '$ui/data/DayCard.svelte';
	import DayGrid from '$ui/data/DayGrid.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import MonthLabel from '$ui/data/MonthLabel.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { isAccessError } from '$lib/api/client';
	import { formatDayCardSubtitle, formatEntryDate, pluralPosts } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadDays } from '$lib/journal/days';
	import { fallbackCoverByDay, loadGrid, photoCountByDay } from '$lib/journal/grid';
	import { groupByMonth } from '$lib/journal/group';
	import type { DaySummary } from '$lib/journal/types';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { markDayPromptSeen } from '$lib/idb/db';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	interface DayView {
		entryDate: string;
		title: string;
		subtitle: string;
		coverBlobId?: string;
		photoCount?: number;
	}

	let days = $state<DayView[]>([]);
	let totalPosts = $state(0);
	let loading = $state(true);
	let error = $state('');
	let coverUrls = $state<Record<string, string>>({});

	async function resolveCovers(items: DayView[]) {
		const next: Record<string, string> = { ...coverUrls };
		for (const day of items) {
			if (!day.coverBlobId || next[day.coverBlobId]) continue;
			try {
				next[day.coverBlobId] = await getMediaUrl(circle.origin, day.coverBlobId);
			} catch {
				/* skip */
			}
		}
		coverUrls = next;
	}

	function buildDayViews(
		summaries: DaySummary[],
		photoCounts: Map<string, number>,
		fallbackCovers: Map<string, string>
	): DayView[] {
		return summaries.map((day) => {
			const coverBlobId = day.cover_blob_id ?? fallbackCovers.get(day.entry_date);
			const photoCount = photoCounts.get(day.entry_date);
			return {
				entryDate: day.entry_date,
				title: day.title || formatEntryDate(day.entry_date),
				subtitle: formatDayCardSubtitle(day.entry_date, day.post_count),
				coverBlobId,
				photoCount: photoCount && photoCount > 1 ? photoCount : undefined
			};
		});
	}

	async function loadData() {
		error = '';
		try {
			const [daysSnap, gridSnap] = await Promise.all([
				loadDays(circle.origin, circle.circleId),
				loadGrid(circle.origin, circle.circleId)
			]);
			totalPosts = daysSnap.days.reduce((sum, d) => sum + d.post_count, 0);
			const views = buildDayViews(
				daysSnap.days,
				photoCountByDay(gridSnap.items),
				fallbackCoverByDay(gridSnap.items)
			);
			days = views;
			await resolveCovers(views);
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить дни';
		} finally {
			loading = false;
		}
	}

	const monthGroups = $derived(groupByMonth(days));

	// Обновление жестом (3.5), как в ленте: тянешь экран от верха.
	let listEl: HTMLDivElement | undefined = $state();
	const ptr = new PullRefresh(() => listEl?.scrollTop ?? 0, () => loadData());
	onDestroy(() => ptr.destroy());

	onMount(() => {
		void markDayPromptSeen(circle.origin, circle.circleId);
		void loadData();
		const unsub = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['days', 'grid'],
			refetch: loadData
		});
		return unsub;
	});

	function goBack() {
		goUp('/circles');
	}

	function openDay(entryDate: string) {
		goto(`/circles/${circle.circleId}/days/${entryDate}`);
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
	active="Дни"
	commentBar={false}
	onback={goBack}
>
	<PullRefreshBand pull={ptr.state} />
	<div
		class="feed"
		role="feed"
		aria-label="Дни"
		bind:this={listEl}
		ontouchstart={ptr.start}
		ontouchmove={ptr.move}
		ontouchend={ptr.end}
	>
	{#if loading}
		<Loading />
	{:else if error && !days.length}
		<Hint class="gutter-24">{error}</Hint>
	{:else}
		{#if days.length}
			<Hint class="mt-12">
				{days.length} {days.length === 1 ? 'день' : days.length < 5 ? 'дня' : 'дней'} из {pluralPosts(totalPosts)}
			</Hint>
		{/if}
		{#each monthGroups as group (group.month)}
			<MonthLabel>{group.label}</MonthLabel>
			<DayGrid>
				{#each group.items as day (day.entryDate)}
					<DayCard
						title={day.title}
						subtitle={day.subtitle}
						coverUrl={day.coverBlobId ? coverUrls[day.coverBlobId] : undefined}
						photoCount={day.photoCount}
						onclick={() => openDay(day.entryDate)}
					/>
				{/each}
			</DayGrid>
		{/each}
		{#if !days.length}
			<Hint class="gutter-24">Пока нет дней с записями</Hint>
		{/if}
	{/if}
	</div>
</CircleLayout>
