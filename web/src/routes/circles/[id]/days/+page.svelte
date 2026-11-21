<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
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
	import { daysSummary } from '$lib/journal/summary-badge';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadDays, loadDaysCached } from '$lib/journal/days';
	import { paintFromSnapshots } from '$lib/journal/snapshot-first';
	import { groupByMonth } from '$lib/journal/group';
	import { dayCoverTileId } from '$lib/journal/present';
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
	// Свежий ответ сети уже на экране — поздний снимок его не перетирает (C100).
	let freshApplied = false;
	let cachedRequested = false;

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

	// Запасную обложку и число фото считает сервер по всем записям дня (C17):
	// снимок «Сетки» ограничен по числу записей и знает только фото.
	function buildDayViews(summaries: DaySummary[]): DayView[] {
		return summaries.map((day) => {
			const coverBlobId = dayCoverTileId(day);
			const photoCount = day.photo_count;
			return {
				entryDate: day.entry_date,
				title: day.title || formatEntryDate(day.entry_date),
				// Безымянный день уже назван датой — второй раз её не пишем (5.1).
				subtitle: day.title
					? formatDayCardSubtitle(day.entry_date, day.post_count)
					: pluralPosts(day.post_count),
				coverBlobId,
				photoCount: photoCount && photoCount > 1 ? photoCount : undefined
			};
		});
	}

	// Снимок дней — первый кадр вкладки (C100-2): «Дни» не ждут сети,
	// свежесть приходит ответом следом.
	async function loadCached() {
		if (cachedRequested) return;
		cachedRequested = true;
		const painted = await paintFromSnapshots(
			() => freshApplied,
			async () => {
				const snap = await loadDaysCached(circle.origin, circle.circleId);
				if (!snap) return undefined;
				return () => {
					totalPosts = snap.days.reduce((sum, d) => sum + d.post_count, 0);
					days = buildDayViews(snap.days);
				};
			}
		);
		if (!painted) return;
		loading = false;
		void resolveCovers(days);
	}

	async function loadData() {
		error = '';
		try {
			const daysSnap = await loadDays(circle.origin, circle.circleId);
			freshApplied = true;
			totalPosts = daysSnap.days.reduce((sum, d) => sum + d.post_count, 0);
			const views = buildDayViews(daysSnap.days);
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
	const ptr = new PullRefresh(
		() => listEl?.scrollTop ?? 0,
		() => loadData()
	);
	onDestroy(() => ptr.destroy());

	onMount(() => {
		void markDayPromptSeen(circle.origin, circle.circleId);
		void (async () => {
			await loadCached();
			await loadData();
		})();
		const unsub = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['days', 'grid'],
			refetch: loadData
		});
		return unsub;
	});

	function goBack() {
		void navigateBack();
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
					{daysSummary(days.length, totalPosts)}
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
