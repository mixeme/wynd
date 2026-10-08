<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { numberParam, withParam, withoutParam } from '$lib/nav/url';
	import { page } from '$app/stores';
	import { getContext, onMount, untrack } from 'svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Lightbox from '$ui/overlays/Lightbox.svelte';
	import VideoDiag from '$ui/overlays/VideoDiag.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import { formatPostTime, pluralPhotos } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import {
		albumCompressionHint,
		albumDownloadFilename,
		findPost,
		lightboxCaption,
		photoMedia,
		tileBlobId
	} from '$lib/journal/present';
	import type { FeedPost, MediaSummary } from '$lib/journal/types';
	import type { MediaSize } from '$lib/journal/present';
	import { downloadBlob, getMediaUrl } from '$lib/media/objectUrl';
	import { keepLocalPoster } from '$lib/media/videoPoster';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const postId = $derived($page.params.postId ?? '');
	const lightboxIndex = $derived(numberParam($page.url, 'lb'));

	let post = $state<FeedPost | undefined>();
	let photos = $state<MediaSummary[]>([]);
	let urls = $state<Record<string, string>>({});
	// Размер снимка знает только сам файл: берём его, когда он открылся.
	let sizes = $state<Record<string, MediaSize>>({});
	let loading = $state(true);

	const currentItem = $derived(photos[lightboxIndex]);
	const albumTitle = $derived(post ? pluralPhotos(photos.length) : '');
	const albumSubtitle = $derived(
		post ? `${post.author_name} · ${formatPostTime(post.created_at, post.entry_date)}` : ''
	);

	onMount(() => {
		void load();
		return registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['feed'],
			refetch: load
		});
	});

	async function load() {
		try {
			const snap = await loadFeed(circle.origin, circle.circleId);
			post = findPost(snap.posts, postId);
			photos = post ? photoMedia(post.media) : [];
			urls = {};
			videoAsked.clear();
			// Пачками: прежний цикл ждал ответа на каждый снимок по очереди.
			// У ролика плитка — его кадр; сам ролик качается, когда его открыли.
			await resolveMediaUrls(
				circle.origin,
				photos.map((p) => tileBlobId(p)),
				(blobId, url) => {
					urls = { ...urls, [blobId]: url };
				}
			);
		} finally {
			loading = false;
		}
	}

	// Ролик качается целиком — на медленной связи подпись говорит, сколько уже есть.
	let videoNote = $state<Record<string, string>>({});
	// Не $state: эффект ниже не должен перезапускаться от собственной работы.
	const videoAsked = new Set<string>();

	async function loadVideo(item: MediaSummary) {
		if (videoAsked.has(item.blob_id)) return;
		videoAsked.add(item.blob_id);
		videoNote = { ...videoNote, [item.blob_id]: 'загрузка…' };
		try {
			const url = await getMediaUrl(circle.origin, item.blob_id, {
				onProgress: (received, total) => {
					if (!total) return;
					const note = `загрузка · ${formatBytes(received)} из ${formatBytes(total)}`;
					videoNote = { ...videoNote, [item.blob_id]: note };
				}
			});
			urls = { ...urls, [item.blob_id]: url };
		} catch {
			videoAsked.delete(item.blob_id);
			videoNote = { ...videoNote, [item.blob_id]: 'не загрузилось' };
		}
	}

	$effect(() => {
		const item = currentItem;
		if (lightboxIndex < 0 || item?.kind !== 'video') return;
		untrack(() => void loadVideo(item));
	});

	// Ролик отправили без кадра: снимаем его здесь, раз файл уже скачан, —
	// плитки на этом устройстве дальше рисуют картинку.
	async function keepPoster(item: MediaSummary) {
		if (item.video_poster_blob_id || urls[tileBlobId(item)]) return;
		const url = await keepLocalPoster(circle.origin, item.blob_id, urls[item.blob_id]);
		if (url) urls = { ...urls, [tileBlobId(item)]: url };
	}

	function noteSize(blobId: string, width: number, height: number) {
		if (width > 0 && height > 0) sizes = { ...sizes, [blobId]: { width, height } };
	}

	function openLightbox(index: number) {
		goto(withParam($page.url.pathname, 'lb', index));
	}

	// Один снимок открывают сразу во весь экран (albumHref, ?single): альбом
	// под ним не нужен — закрытие возвращает туда, откуда открыли.
	const single = $derived($page.url.searchParams.has('single'));
	let openedFrom = '';
	afterNavigate(({ from }) => {
		if (!openedFrom && from) openedFrom = from.url.pathname + from.url.search;
	});

	function closeLightbox() {
		if (single) {
			void goUp(openedFrom || `/circles/${circle.circleId}/posts/${postId}`);
			return;
		}
		const back = withoutParam($page.url, 'lb');
		if (back) goto(back);
	}

	function downloadCurrent() {
		const item = photos[lightboxIndex];
		if (!item) return;
		void downloadBlob(
			circle.origin,
			item.blob_id,
			albumDownloadFilename(item, lightboxIndex)
		);
	}
</script>

<CircleLayout
	app
	color={circle.color}
	title={loading ? '…' : albumTitle || 'Альбом'}
	subtitle={post ? albumSubtitle : undefined}
	tabs={false}
	commentBar={false}
	onback={() => goUp(`/circles/${circle.circleId}/posts/${postId}`)}
>
	{#if loading}
		<Loading />
	{:else if !post}
		<Hint class="gutter-24">Запись не найдена</Hint>
	{:else}
		<PhotoGrid class="mt-3 gap-4" album>
			{#each photos as photo, i (photo.blob_id)}
				<MediaTile
					variant="album"
					src={urls[tileBlobId(photo)]}
					kind={photo.kind === 'video' ? 'video' : 'photo'}
					coverLabel={photo.is_cover ? 'обложка' : undefined}
					onclick={() => openLightbox(i)}
				/>
			{/each}
		</PhotoGrid>
		{#if post}
			<Hint class="m-16-16-0 ctr-text">
				{albumCompressionHint(post)}
			</Hint>
		{/if}
	{/if}
</CircleLayout>

{#if lightboxIndex >= 0 && currentItem && post}
	{@const item = currentItem}
	<Lightbox
		fixed
		counter="{lightboxIndex + 1} из {photos.length}"
		caption={item.kind === 'video' && !urls[item.blob_id]
			? videoNote[item.blob_id]
			: lightboxCaption(post, item, formatPostTime, sizes[item.blob_id])}
		dotCount={photos.length}
		dotIndex={lightboxIndex}
		onDotSelect={openLightbox}
		onclose={closeLightbox}
		ondownload={item.kind === 'photo' || item.kind === 'video' ? downloadCurrent : undefined}
		onprev={lightboxIndex > 0 ? () => openLightbox(lightboxIndex - 1) : undefined}
		onnext={lightboxIndex < photos.length - 1 ? () => openLightbox(lightboxIndex + 1) : undefined}
	>
		{#snippet media()}
			{#if item.kind === 'video'}
				{#if urls[item.blob_id]}
					<!-- playsinline: на iPhone видео играет в лайтбоксе, а не уходит в полноэкранный плеер. -->
					<video
						src={urls[item.blob_id]}
						poster={urls[tileBlobId(item)]}
						controls
						playsinline
						preload="metadata"
						onloadedmetadata={(e) => noteSize(item.blob_id, e.currentTarget.videoWidth, e.currentTarget.videoHeight)}
						onloadeddata={() => void keepPoster(item)}
					>
						<track kind="captions" label="Субтитры отсутствуют" />
					</video>
				{:else if urls[tileBlobId(item)]}
					<img src={urls[tileBlobId(item)]} alt="" />
				{/if}
			{:else}
				<img
					src={urls[item.blob_id]}
					alt=""
					onload={(e) => {
						const img = e.currentTarget as HTMLImageElement;
						noteSize(item.blob_id, img.naturalWidth, img.naturalHeight);
					}}
				/>
			{/if}
		{/snippet}
	</Lightbox>
	{#if item.kind === 'video'}
		<!-- Временно (план 46, C26): вход на страницу диагностики видео. -->
		<VideoDiag
			link="/circles/{circle.circleId}/posts/{postId}/album/diag?m={encodeURIComponent(item.blob_id)}&p={encodeURIComponent(tileBlobId(item))}"
		/>
	{/if}
{/if}
