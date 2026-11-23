<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatEntryDate } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import {
		loadDay,
		loadDayCached,
		loadDays,
		loadDaysCached,
		setDayCover,
		clearDayCover
	} from '$lib/journal/days';
	import { paintFromSnapshots } from '$lib/journal/snapshot-first';
	import type { FeedPost } from '$lib/journal/types';
	import { attachmentMedia, photoMedia, tileBlobId } from '$lib/journal/present';
	import { getMediaUrl } from '$lib/media/objectUrl';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const entryDate = $derived($page.params.date ?? '');

	interface AlbumItem {
		postId: string;
		blobId: string;
		kind: string;
		durationMs?: number;
		preview?: string;
	}

	let items = $state<AlbumItem[]>([]);
	let selected = $state<{ postId: string; blobId: string } | undefined>();
	let coverBlobId = $state<string | undefined>();
	let coverPostId = $state<string | undefined>();
	let ownPostThatDay = $state(false);
	let loading = $state(true);
	let saving = $state(false);
	let clearing = $state(false);
	let error = $state('');
	// Свежий ответ сети уже на экране — поздний снимок его не перетирает (C100).
	let freshApplied = false;
	let cachedRequested = false;
	// Снимок нарисован: сеть уточняет плитки, но не прячет их загрузкой (C105).
	let cachedShown = false;

	// Окна правок у обложки нет: убрать — такая же запись журнала, как выбрать.
	const canClearCover = $derived(Boolean(coverPostId) && ownPostThatDay);

	onMount(() => {
		// Альбом дня — выбор обложки; читателю он не нужен (SCR-2).
		if (!circle.canWrite || circle.spaceFull) {
			goto(`/circles/${circle.circleId}/days/${entryDate}${$page.url.search}`, {
				replaceState: true
			});
			return;
		}
		void (async () => {
			await loadCached();
			await load();
		})();
	});

	// Плитки альбома из записей дня: фото, ролики и обложки звуков.
	async function buildItems(posts: FeedPost[]): Promise<AlbumItem[]> {
		const next: AlbumItem[] = [];
		for (const post of posts) {
			for (const m of photoMedia(post.media)) {
				next.push({
					postId: post.id,
					blobId: m.blob_id,
					kind: m.kind,
					durationMs: m.video_duration_ms,
					// Ролик без кадра выбрать можно, но показать нечего.
					preview: await getMediaUrl(circle.origin, tileBlobId(m)).catch(() => undefined)
				});
			}
			// Обложка звука тоже годится в обложку дня.
			for (const a of attachmentMedia(post.media)) {
				const art = a.audio_cover_blob_id;
				if (!art || next.some((i) => i.blobId === art)) continue;
				next.push({
					postId: post.id,
					blobId: art,
					kind: 'photo',
					preview: await getMediaUrl(circle.origin, art)
				});
			}
		}
		return next;
	}

	// Снимки дня и сводки дней — первый кадр альбома (C100-5): плитки и
	// выбранная обложка на месте до ответа сети.
	async function loadCached() {
		if (cachedRequested) return;
		cachedRequested = true;
		const painted = await paintFromSnapshots(
			() => freshApplied,
			async () => {
				const [day, daysSnap] = await Promise.all([
					loadDayCached(circle.origin, circle.circleId, entryDate).catch(() => undefined),
					loadDaysCached(circle.origin, circle.circleId).catch(() => undefined)
				]);
				if (!day && !daysSnap) return undefined;
				const next = day ? await buildItems(day.posts) : [];
				const meta = daysSnap?.days.find((d) => d.entry_date === entryDate);
				return () => {
					if (day) {
						items = next;
						ownPostThatDay = day.posts.some((p) => p.identity_id === circle.identityId);
					}
					if (meta) {
						coverBlobId = meta.cover_blob_id;
						coverPostId = meta.cover_post_id;
					}
					const current = next.find((i) => i.blobId === coverBlobId) ?? next[0];
					if (current) selected = { postId: current.postId, blobId: current.blobId };
				};
			}
		);
		if (!painted) return;
		cachedShown = true;
		loading = false;
	}

	async function load() {
		// Снимок уже нарисован — плитки остаются на экране, загрузку не показываем.
		if (!cachedShown) loading = true;
		error = '';
		try {
			const [day, daysSnap] = await Promise.all([
				loadDay(circle.origin, circle.circleId, entryDate),
				loadDays(circle.origin, circle.circleId)
			]);
			freshApplied = true;
			const meta = daysSnap.days.find((d) => d.entry_date === entryDate);
			coverBlobId = meta?.cover_blob_id;
			coverPostId = meta?.cover_post_id;
			ownPostThatDay = day.posts.some((p) => p.identity_id === circle.identityId);
			const next = await buildItems(day.posts);
			items = next;
			const current = next.find((i) => i.blobId === coverBlobId) ?? next[0];
			if (current) selected = { postId: current.postId, blobId: current.blobId };
		} catch {
			error = 'Не удалось загрузить медиа дня';
		} finally {
			loading = false;
		}
	}

	function goBack() {
		void navigateBack();
	}

	function selectItem(item: (typeof items)[number]) {
		selected = { postId: item.postId, blobId: item.blobId };
	}

	async function save() {
		if (!selected) return;
		saving = true;
		error = '';
		try {
			await setDayCover(
				circle.origin,
				circle.circleId,
				entryDate,
				selected.postId,
				selected.blobId
			);
			goBack();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			saving = false;
		}
	}

	async function clearCover() {
		clearing = true;
		error = '';
		try {
			await clearDayCover(circle.origin, circle.circleId, entryDate);
			goBack();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			clearing = false;
		}
	}
</script>

<!-- Шапка «‹ · Обложка дня · Сохранить» в цвет круга (C44, слово владельца).
     Была стрелка «Назад» и рядом «Отмена» — две кнопки на одно действие, без
     названия экрана. Правило C98: обложка дня — редактор 48 (решение 21.11). -->
<FormLayout
	app
	compose
	cancelBack
	compact
	color={circle.color}
	title="Обложка дня"
	publishLabel="Сохранить"
	canPublish={Boolean(selected) && !saving}
	publishing={saving}
	oncancel={goBack}
	onpublish={() => void save()}
	onback={goBack}
>
	<Hint class="m-8-16-8">{formatEntryDate(entryDate)}</Hint>

	{#if loading}
		<Loading />
	{:else if !items.length}
		<Hint class="gutter">В этот день нет фото, видео или обложек звука</Hint>
	{:else}
		<PhotoGrid album>
			{#each items as item (item.blobId)}
				<MediaTile
					variant="album"
					src={item.preview}
					kind={item.kind === 'video' ? 'video' : 'photo'}
					durationMs={item.durationMs}
					selected={selected?.blobId === item.blobId}
					onclick={() => selectItem(item)}
				/>
			{/each}
		</PhotoGrid>
	{/if}
	{#if canClearCover}
		<Hint class="mt-8" centered>
			<TextButton disabled={clearing} onclick={() => void clearCover()}>убрать обложку</TextButton>
		</Hint>
	{/if}

	{#if error}
		<Hint class="gutter-8">{error}</Hint>
	{/if}
</FormLayout>
