<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { numberParam, withParam, withoutParam } from '$lib/nav/url';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Lightbox from '$ui/overlays/Lightbox.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { formatPostTime, pluralPhotos } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import {
		albumCompressionHint,
		albumDownloadFilename,
		findPost,
		lightboxCaption,
		photoMedia
	} from '$lib/journal/present';
	import type { FeedPost, MediaSummary } from '$lib/journal/types';
	import type { MediaSize } from '$lib/journal/present';
	import { downloadBlob } from '$lib/media/objectUrl';
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
			// Пачками: прежний цикл ждал ответа на каждый снимок по очереди.
			await resolveMediaUrls(
				circle.origin,
				photos.map((p) => p.blob_id),
				(blobId, url) => {
					urls = { ...urls, [blobId]: url };
				}
			);
		} finally {
			loading = false;
		}
	}

	function noteSize(blobId: string, width: number, height: number) {
		if (width > 0 && height > 0) sizes = { ...sizes, [blobId]: { width, height } };
	}

	function openLightbox(index: number) {
		goto(withParam($page.url.pathname, 'lb', index));
	}

	function closeLightbox() {
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
		<PhotoGrid album style="margin-top:3px;gap:4px">
			{#each photos as photo, i (photo.blob_id)}
				<MediaTile
					variant="album"
					src={urls[photo.blob_id]}
					kind={photo.kind === 'video' ? 'video' : 'photo'}
					coverLabel={photo.is_cover ? 'обложка' : undefined}
					onclick={() => openLightbox(i)}
				/>
			{/each}
		</PhotoGrid>
		{#if post}
			<Hint style="margin:16px 16px 0;text-align:center">
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
		caption={lightboxCaption(post, item, formatPostTime, sizes[item.blob_id])}
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
				<!-- playsinline: на iPhone видео играет в лайтбоксе, а не уходит в полноэкранный плеер. -->
				<video
					src={urls[item.blob_id]}
					controls
					playsinline
					preload="metadata"
					onloadedmetadata={(e) => noteSize(item.blob_id, e.currentTarget.videoWidth, e.currentTarget.videoHeight)}
				>
					<track kind="captions" label="Субтитры отсутствуют" />
				</video>
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
{/if}
