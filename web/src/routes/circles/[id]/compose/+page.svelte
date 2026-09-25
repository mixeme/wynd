<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatEditableUntil, formatEntryDate, formatPostTime, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeedCached } from '$lib/journal/feed';
	import { findPost } from '$lib/journal/present';
	import { createPost, deletePost, editPost as savePost, fetchCompression, uploadBlob } from '$lib/journal/posts';
	import type { FeedPost, MediaSummary } from '$lib/journal/types';
	import { compressImage, fileToQueueBuffer, isImageFile, isVideoFile } from '$lib/media/compress';
	import { readExif } from '$lib/media/exif';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { enqueuePost, loadQueuedPost, removeQueueItem, updateQueuedPost } from '$lib/queue/queue';
	import type { PostQueuePayload, QueueFile, QueueMediaMeta } from '$lib/idb/db';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const editPostId = $derived($page.url.searchParams.get('post'));
	const editQueueId = $derived(Number($page.url.searchParams.get('queue') || 0) || 0);
	const isEdit = $derived(Boolean(editPostId));
	const isQueue = $derived(Boolean(editQueueId));

	let body = $state('');
	let entryDate = $state('');
	let entryDateFromExif = $state(false);
	let loading = $state(false);
	let error = $state('');
	let editingPost = $state<FeedPost | undefined>();
	let picked = $state<
		Array<{
			preview?: string;
			file?: QueueFile;
			blobId?: string;
			meta: QueueMediaMeta;
		}>
	>([]);

	let bodyInput: HTMLTextAreaElement | undefined = $state();
	let photoInput: HTMLInputElement | undefined = $state();
	let attachInput: HTMLInputElement | undefined = $state();
	let dateInput: HTMLInputElement | undefined = $state();

	const canPublish = $derived(Boolean(body.trim()) || picked.length > 0);
	const showEditWindowNote = $derived(
		isEdit &&
			editingPost &&
			editingPost.edit_window_sec !== circle.editWindowSec &&
			!(editingPost.edit_window_sec == null && circle.editWindowSec == null)
	);
	const entryDateSubtitle = $derived(
		entryDate
			? entryDateFromExif
				? `${formatEntryDate(entryDate)} · взято из снимка`
				: formatEntryDate(entryDate)
			: ''
	);
	const barTitle = $derived(isEdit ? 'Правка' : circle.name);

	function today(): string {
		const d = new Date();
		return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	}

	function composeDraftKey(): string {
		return `wynd.compose.${circle.circleId}`;
	}

	function editWindowSubtitle(post: FeedPost): string {
		const when = formatPostTime(post.created_at, post.entry_date).split(',')[0];
		const sec = post.edit_window_sec;
		if (sec == null) return `вышла ${when}, когда в круге не было ограничения`;
		if (sec === 86_400) return `вышла ${when}, когда в круге стояли сутки`;
		if (sec === 3600) return `вышла ${when}, когда в круге стоял час`;
		const hours = Math.round(sec / 3600);
		if (hours < 24) {
			return `вышла ${when}, когда в круге стоял${hours === 1 ? '' : 'и'} ${hours} ч`;
		}
		const days = Math.round(hours / 24);
		return `вышла ${when}, когда в круге стояли ${days === 1 ? 'сутки' : `${days} суток`}`;
	}

	function openDatePicker() {
		dateInput?.showPicker?.() ?? dateInput?.click();
	}

	function resizeBody() {
		if (!bodyInput) return;
		bodyInput.style.height = 'auto';
		bodyInput.style.height = `${bodyInput.scrollHeight}px`;
	}

	onMount(async () => {
		entryDate = today();
		if (editPostId) {
			const cached = await loadFeedCached(circle.origin, circle.circleId);
			const post = cached ? findPost(cached.posts, editPostId) : undefined;
			if (post) {
				editingPost = post;
				body = post.body;
				entryDate = post.entry_date;
				const items = [];
				for (const m of post.media ?? []) {
					const url = await getMediaUrl(circle.origin, m.blob_id);
					items.push({
						preview: url,
						blobId: m.blob_id,
						meta: { kind: m.kind, is_cover: m.is_cover }
					});
				}
				picked = items;
			}
		} else if (editQueueId) {
			const item = await loadQueuedPost(editQueueId);
			if (item) {
				const payload = item.payload as PostQueuePayload;
				body = payload.body;
				entryDate = payload.entry_date;
				picked = item.files.map((file, i) => {
					const meta = payload.media_meta?.[i] ?? { kind: 'photo' };
					const preview =
						meta.kind === 'photo' || meta.kind === 'video'
							? URL.createObjectURL(new Blob([file.data], { type: file.type }))
							: undefined;
					return { preview, file, meta };
				});
			}
		} else {
			const draft = sessionStorage.getItem(composeDraftKey());
			if (draft) {
				body = draft;
				sessionStorage.removeItem(composeDraftKey());
			}
		}
		bodyInput?.focus();
		resizeBody();
	});

	function feedHref(dayPrompt?: string) {
		const base = `/circles/${circle.circleId}`;
		return dayPrompt ? `${base}?dayPrompt=${dayPrompt}` : base;
	}

	function goBack() {
		goto(feedHref());
	}

	async function deleteDraft() {
		if (!editQueueId) return;
		await removeQueueItem(editQueueId);
		goto(feedHref());
	}

	async function deleteEditedPost() {
		if (!editPostId) return;
		loading = true;
		error = '';
		try {
			await deletePost(circle.origin, circle.circleId, editPostId);
			goto(feedHref());
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	function isVisual(meta: QueueMediaMeta): boolean {
		return meta.kind === 'photo' || meta.kind === 'video';
	}

	function setCover(index: number) {
		picked = picked.map((item, i) => ({
			...item,
			meta: {
				...item.meta,
				is_cover: i === index && isVisual(item.meta)
			}
		}));
	}

	function coverBlobId(): string | undefined {
		return picked.find((p) => p.meta.is_cover && p.blobId)?.blobId;
	}

	async function onFilesSelected(e: Event) {
		const input = e.target as HTMLInputElement;
		const files = input.files;
		if (!files?.length) return;

		const compression = await fetchCompression(circle.origin).catch(() => undefined);
		const next = [...picked];

		for (const file of files) {
			const exif = await readExif(file);
			let queueFile: QueueFile;
			if (isImageFile(file)) {
				queueFile = await compressImage(file, compression);
			} else {
				queueFile = await fileToQueueBuffer(file);
			}

			const kind = isVideoFile(file) ? 'video' : isImageFile(file) ? 'photo' : 'attachment';

			const meta: QueueMediaMeta = {
				kind,
				captured_at: exif.captured_at,
				geo_lat: exif.geo_lat,
				geo_lng: exif.geo_lng,
				is_cover:
					(kind === 'photo' || kind === 'video') &&
					!next.some((n) => n.meta.is_cover && (n.meta.kind === 'photo' || n.meta.kind === 'video'))
			};

			if (exif.entry_date && entryDate === today()) {
				entryDate = exif.entry_date;
				entryDateFromExif = true;
			}

			const preview =
				kind === 'photo' || kind === 'video'
					? URL.createObjectURL(new Blob([queueFile.data], { type: queueFile.type }))
					: undefined;

			next.push({ preview, file: queueFile, meta });
		}
		picked = next;
		input.value = '';
	}

	async function publish() {
		error = '';
		const trimmed = body.trim();
		if (!trimmed && !picked.length) {
			error = 'Добавьте текст или вложение';
			return;
		}
		loading = true;
		try {
			if (isEdit && editPostId) {
				const cover = coverBlobId();
				await savePost(circle.origin, circle.circleId, editPostId, trimmed, entryDate, cover);
				goto(feedHref());
				return;
			}

			const files = picked.map((p) => p.file!).filter(Boolean);
			const payload = {
				body: trimmed,
				entry_date: entryDate,
				media_meta: picked.map((p) => p.meta)
			};

			if (editQueueId) {
				await updateQueuedPost(editQueueId, payload, files);
				goto(feedHref());
				return;
			}

			if (!navigator.onLine) {
				await enqueuePost(circle.origin, circle.circleId, payload, files);
				goto(feedHref());
				return;
			}

			const media: MediaSummary[] = [];
			for (const item of picked) {
				if (!item.file) continue;
				const blobId = await uploadBlob(circle.origin, item.file);
				media.push({
					blob_id: blobId,
					kind: item.meta.kind as MediaSummary['kind'],
					captured_at: item.meta.captured_at,
					geo_lat: item.meta.geo_lat,
					geo_lng: item.meta.geo_lng,
					is_cover: item.meta.is_cover ?? false
				});
			}

			await createPost(circle.origin, circle.circleId, {
				body: trimmed,
				entry_date: entryDate,
				captured_at: picked[0]?.meta.captured_at,
				media
			});
			goto(feedHref(entryDate));
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}
</script>

<FormLayout
	app
	compose
	color={circle.color}
	title={barTitle}
	publishLabel={isEdit ? 'Сохранить' : 'Опубликовать'}
	canPublish={canPublish}
	publishing={loading}
	oncancel={goBack}
	onpublish={() => void publish()}
	footer={composeFooter}
>
	<div class="compose-body">
		<TextArea
			variant="compose"
			bind:el={bodyInput}
			bind:value={body}
			placeholder="Что случилось?"
			rows={1}
			oninput={resizeBody}
		/>

		<div class="thumbs">
			{#each picked as item, i (i)}
				<button
					type="button"
					class="thumb"
					class:cover={item.meta.is_cover && isVisual(item.meta)}
					onclick={() => isVisual(item.meta) && setCover(i)}
				>
					{#if item.preview}
						{#if item.meta.kind === 'video'}
							<video src={item.preview} muted playsinline></video>
						{:else}
							<img src={item.preview} alt="" />
						{/if}
					{:else}
						<span class="file">{item.file?.name}</span>
					{/if}
				</button>
			{/each}
			<AddPhotoButton onclick={() => photoInput?.click()} />
		</div>

		{#if !isEdit && picked.some((p) => isVisual(p.meta))}
			<Hint>Обложка — первая. Нажмите на другую, чтобы лента показывала её.</Hint>
		{/if}

		{#if !isEdit}
			<input bind:this={dateInput} type="date" bind:value={entryDate} hidden />
			<SettingsRow
				icon="clock"
				title="Отнести к дате"
				subtitle={entryDateSubtitle}
				style="margin-top:16px;border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
				onclick={openDatePicker}
			/>
			<Hint>
				В ленте запись всё равно встанет сегодняшним числом. Дата нужна дню — в «Днях» она
				соберёт её с остальными за {entryDate ? formatEntryDate(entryDate) : 'этот день'}.
			</Hint>
		{/if}

		{#if showEditWindowNote && editingPost?.editable_until}
			<SettingsRow
				icon="clock"
				title="Правится до {formatEditableUntil(editingPost.editable_until)}"
				subtitle={editWindowSubtitle(editingPost)}
				chevron={false}
				style="margin-top:18px;border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
			/>
			<Hint>
				Сейчас в круге стоит {circle.editWindowSec === 3600
					? 'час'
					: circle.editWindowSec === 86_400
						? 'сутки'
						: circle.editWindowSec == null
							? 'без ограничения'
							: `${Math.round((circle.editWindowSec ?? 0) / 3600)} ч`}, но запись сохранила своё
				окно: правило поменяли после неё. У новых записей будет иначе.
			</Hint>
		{/if}

		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}

		{#if isEdit && editingPost && isEditableActive(editingPost.editable_until)}
			<div style="margin-top:20px">
				<DangerZone items={['Удалить запись']} onitem={() => void deleteEditedPost()} />
			</div>
			<Hint style="margin:8px 16px 0">
				Удаление живёт по тому же окну: выйдет срок — исчезнет и эта строка.
			</Hint>
		{/if}

		{#if isQueue}
			<div class="hint" style="margin:16px">
				<TextButton onclick={deleteDraft}>Удалить черновик</TextButton>
			</div>
		{/if}
	</div>
</FormLayout>

{#snippet composeFooter()}
	<div class="compose-bar">
		<div class="tools">
			<IconButton
				name="photo"
				label="Фото или видео"
				disabled={isEdit}
				onclick={() => photoInput?.click()}
			/>
			<IconButton name="file" label="Файл" disabled={isEdit} onclick={() => attachInput?.click()} />
			<span class="who">пишете как {circle.identityName}</span>
		</div>
	</div>
{/snippet}

<input
	bind:this={photoInput}
	type="file"
	accept="image/*,video/*"
	multiple
	hidden
	onchange={onFilesSelected}
/>
<input bind:this={attachInput} type="file" accept="*/*" multiple hidden onchange={onFilesSelected} />

<style>
	.thumbs {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		padding: 16px 16px 0;
		align-items: flex-start;
	}
	.thumb {
		width: 88px;
		height: 88px;
		border-radius: 8px;
		overflow: hidden;
		background: var(--tint);
		border: none;
		padding: 0;
		cursor: pointer;
	}
	.thumb.cover {
		outline: 2px solid var(--c);
		outline-offset: 2px;
	}
	.thumb img,
	.thumb video {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.file {
		font-size: 10px;
		padding: 4px;
		word-break: break-all;
	}
</style>
