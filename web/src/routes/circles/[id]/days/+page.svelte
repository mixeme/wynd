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
	import { isVideoMime } from '$lib/journal/present';
	import { groupByMonth } from '$lib/journal/group';
	import type { DaySummary } from '$lib/journal/types';
	import { coverImageIsBlank } from '$lib/media/audioTags';
	import { loadMedia } from '$lib/media/objectUrl';
	import { markDayPromptSeen } from '$lib/idb/db';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	interface DayView {
		entryDate: string;
		title: string;
		subtitle: string;
		coverBlobId?: string;
		/** Сам файл, если coverBlobId — картинка кадра. Чёрный кадр меняем на ролик. */
		coverFileId?: string;
		photoCount?: number;
	}

	let days = $state<DayView[]>([]);
	let totalPosts = $state(0);
	let loading = $state(true);
	let error = $state('');
	let coverUrls = $state<Record<string, string>>({});
	let coverVideo = $state<Record<string, boolean>>({});

	async function resolveCovers(items: DayView[]) {
		const next: Record<string, string> = { ...coverUrls };
		const video: Record<string, boolean> = { ...coverVideo };
		for (const day of items) {
			if (!day.coverBlobId || next[day.coverBlobId]) continue;
			try {
				const media = await loadMedia(circle.origin, day.coverBlobId);
				let url = media.url;
				let asVideo = isVideoMime(media.mime);
				if (
					!asVideo &&
					day.coverFileId &&
					day.coverFileId !== day.coverBlobId &&
					(await coverImageIsBlank(url))
				) {
					const file = await loadMedia(circle.origin, day.coverFileId);
					url = file.url;
					asVideo = true;
				}
				next[day.coverBlobId] = url;
				video[day.coverBlobId] = asVideo;
			} catch {
				/* skip */
			}
		}
		coverUrls = next;
		coverVideo = video;
	}

	// Запасную обложку и число фото считает сервер по всем записям дня (C17).
	function buildDayViews(summaries: DaySummary[]): DayView[] {
		return summaries.map((day) => {
			const fileId = day.cover_blob_id ?? day.fallback_cover_blob_id;
			const coverBlobId = day.cover_image_blob_id ?? fileId;
			const photoCount = day.photo_count;
			return {
				entryDate: day.entry_date,
				title: day.title || formatEntryDate(day.entry_date),
				subtitle: formatDayCardSubtitle(day.entry_date, day.post_count),
				coverBlobId,
				coverFileId: day.cover_image_blob_id ? fileId : undefined,
				photoCount: photoCount && photoCount > 1 ? photoCount : undefined
			};
		});
	}

	async function loadData() {
		error = '';
		try {
			const daysSnap = await loadDays(circle.origin, circle.circleId);
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
						kind={day.coverBlobId && coverVideo[day.coverBlobId] ? 'video' : 'photo'}
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
