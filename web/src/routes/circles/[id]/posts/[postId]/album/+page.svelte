<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Lightbox from '$ui/overlays/Lightbox.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { formatPostTime } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import { albumCompressionHint, findPost, lightboxCaption, photoMedia } from '$lib/journal/present';
	import { fetchCompression } from '$lib/journal/posts';
	import type { FeedPost, MediaSummary } from '$lib/journal/types';
	import { downloadBlob, getMediaUrl } from '$lib/media/objectUrl';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const postId = $derived($page.params.postId ?? '');
	const lightboxIndex = $derived(Number($page.url.searchParams.get('lb') ?? -1));

	let post = $state<FeedPost | undefined>();
	let photos = $state<MediaSummary[]>([]);
	let urls = $state<Record<string, string>>({});
	let photoMaxPx = $state<number | undefined>();
	let loading = $state(true);

	const currentItem = $derived(photos[lightboxIndex]);

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
			const [snap, compression] = await Promise.all([
				loadFeed(circle.origin, circle.circleId),
				fetchCompression(circle.origin).catch(() => undefined)
			]);
			post = findPost(snap.posts, postId);
			photos = post ? photoMedia(post.media) : [];
			photoMaxPx = compression?.photo_max_px;
			const next: Record<string, string> = {};
			for (const p of photos) {
				next[p.blob_id] = await getMediaUrl(circle.origin, p.blob_id);
			}
			urls = next;
		} finally {
			loading = false;
		}
	}

	function openLightbox(index: number) {
		goto(`/circles/${circle.circleId}/posts/${postId}/album?lb=${index}`);
	}

	function closeLightbox() {
		goto(`/circles/${circle.circleId}/posts/${postId}/album`);
	}

	function downloadCurrent() {
		const photo = photos[lightboxIndex];
		if (!photo) return;
		void downloadBlob(circle.origin, photo.blob_id, `photo-${lightboxIndex + 1}.jpg`);
	}
</script>

<FormLayout
	app
	color={circle.color}
	title="Альбом"
	onback={() => goto(`/circles/${circle.circleId}/posts/${postId}`)}
>
	{#if loading}
		<Hint style="margin:24px 16px">Загрузка…</Hint>
	{:else if !post}
		<Hint style="margin:24px 16px">Запись не найдена</Hint>
	{:else}
		<div class="sub" style="padding:0 16px 8px;font-size:12.5px;color:var(--muted)">
			{post.author_name} · {formatPostTime(post.created_at, post.entry_date)}
		</div>
		<div class="g3" style="padding:0 12px 16px">
			{#each photos as photo, i (photo.blob_id)}
				<button type="button" class="cell" onclick={() => openLightbox(i)}>
					{#if urls[photo.blob_id]}
						{#if photo.kind === 'video'}
							<video src={urls[photo.blob_id]} muted playsinline></video>
						{:else}
							<img src={urls[photo.blob_id]} alt="" />
						{/if}
					{/if}
					{#if photo.is_cover}
						<span class="cov">обложка</span>
					{/if}
				</button>
			{/each}
		</div>
		{#if post}
			<Hint style="margin:16px 16px 0;text-align:center">
				{albumCompressionHint(post, photoMaxPx)}
			</Hint>
		{/if}
	{/if}
</FormLayout>

{#if lightboxIndex >= 0 && currentItem && post}
	{@const item = currentItem}
	<Lightbox
		counter="{lightboxIndex + 1} из {photos.length}"
		caption={lightboxCaption(post, item, formatPostTime)}
		onclose={closeLightbox}
		ondownload={item.kind === 'photo' ? downloadCurrent : undefined}
		onprev={lightboxIndex > 0 ? () => openLightbox(lightboxIndex - 1) : undefined}
		onnext={lightboxIndex < photos.length - 1 ? () => openLightbox(lightboxIndex + 1) : undefined}
	>
		{#snippet media()}
			{#if item.kind === 'video'}
				<video src={urls[item.blob_id]} controls></video>
			{:else}
				<img src={urls[item.blob_id]} alt="" />
			{/if}
		{/snippet}
		{#snippet dots()}
			{#each photos as _, i (i)}
				<u class:on={i === lightboxIndex} onclick={() => openLightbox(i)}></u>
			{/each}
		{/snippet}
	</Lightbox>
{/if}

<style>
	.g3 {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 4px;
	}
	.cell {
		position: relative;
		aspect-ratio: 1;
		overflow: hidden;
		border-radius: 4px;
		background: var(--tint);
		border: none;
		padding: 0;
		cursor: pointer;
	}
	.cell img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.cell video {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.cov {
		position: absolute;
		left: 4px;
		bottom: 4px;
		font-size: 10px;
		background: rgba(0, 0, 0, 0.45);
		color: #fff;
		padding: 2px 4px;
		border-radius: 3px;
	}
	:global(.lb) {
		z-index: 50;
		position: fixed;
		inset: 0;
	}
</style>
