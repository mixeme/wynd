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
	import MediaTile from '$ui/data/MediaTile.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import MentionPicker from '$ui/forms/MentionPicker.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { memberAvatarColor } from '$lib/auth/invites';
	import { formatBytes } from '$lib/format/bytes';
	import { authErrorHint } from '$lib/auth/auth';
	import { circleInitial } from '$lib/circles/meta';
	import { fetchMembers, type MemberInfo } from '$lib/circles/settings';
	import { formatEditableUntil, formatEntryDate, formatPostTime, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeedCached } from '$lib/journal/feed';
	import {
		filterMembersByMention,
		insertMention,
		MAX_TEXT_BYTES,
		mentionQueryAt,
		textByteLength
	} from '$lib/journal/mentions';
	import { findPost } from '$lib/journal/present';
	import { createPost, deletePost, editPost as savePost, fetchCompression, uploadBlob } from '$lib/journal/posts';
	import type { FeedPost, MediaSummary } from '$lib/journal/types';
	import {
		compressImage,
		compressVideo,
		fileToQueueBuffer,
		isImageFile,
		isLargeVideo,
		isVideoFile
	} from '$lib/media/compress';
	import { readExif } from '$lib/media/exif';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { enqueuePost, loadQueuedPost, removeQueueItem, updateQueuedPost } from '$lib/queue/queue';
	import { isTransportError } from '$lib/queue/transport';
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
			fileName?: string;
			meta: QueueMediaMeta;
		}>
	>([]);

	let bodyInput: HTMLTextAreaElement | undefined = $state();
	let photoInput: HTMLInputElement | undefined = $state();
	let attachInput: HTMLInputElement | undefined = $state();
	let dateInput: HTMLInputElement | undefined = $state();
	let members = $state<MemberInfo[]>([]);
	let mentionStart = $state<number | null>(null);
	let mentionQuery = $state('');
	let compressing = $state(false);
	let compressProgress = $state(0);
	let compressIndex = $state(0);
	let compressTotal = $state(0);
	let keepOpenHint = $state(false);
	let filePickLock = false;

	const activeMembers = $derived(members.filter((m) => m.status === 'active'));
	const mentionCandidates = $derived(
		mentionStart == null ? [] : filterMembersByMention(activeMembers, mentionQuery)
	);
	const showMentionPicker = $derived(mentionStart != null && mentionCandidates.length > 0);

	const canPublish = $derived(
		!compressing && (Boolean(body.trim()) || picked.length > 0)
	);
	const compressHint = $derived.by(() => {
		if (!compressing) return '';
		const pct = Math.round(compressProgress * 100);
		const of =
			compressTotal > 1 ? ` (${compressIndex} из ${compressTotal})` : '';
		return pct > 0 ? `Сжимаем видео${of}… ${pct}%` : `Сжимаем видео${of}…`;
	});
	const windowsDiverged = $derived(
		isEdit &&
			editingPost &&
			editingPost.edit_window_sec !== circle.editWindowSec &&
			!(editingPost.edit_window_sec == null && circle.editWindowSec == null)
	);
	const editWindowTitle = $derived(
		editingPost?.editable_until?.startsWith('9999-')
			? 'Правится без ограничения'
			: editingPost?.editable_until
				? `Правится до ${formatEditableUntil(editingPost.editable_until)}`
				: ''
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

	// Черновик новой записи живёт в sessionStorage, пока её не опубликовали:
	// телефон выгружает вкладку, пока открыта камера, и набранное в compose
	// пропадало — сохранялся только текст из полосы ленты (план 42, SCR-3).
	// Правка опубликованной или отложенной записи черновик не трогает.
	let draftReady = $state(false);

	function readDraft(): string {
		try {
			return sessionStorage.getItem(composeDraftKey()) ?? '';
		} catch {
			return '';
		}
	}

	function clearDraft() {
		try {
			sessionStorage.removeItem(composeDraftKey());
		} catch {
			/* хранилище недоступно — черновика и не было */
		}
	}

	$effect(() => {
		if (!draftReady) return;
		const text = body;
		try {
			if (text.trim()) sessionStorage.setItem(composeDraftKey(), text);
			else sessionStorage.removeItem(composeDraftKey());
		} catch {
			/* приватный режим без хранилища — черновик просто не сохраняется */
		}
	});

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
		if (!dateInput) return;
		try {
			dateInput.showPicker();
		} catch {
			dateInput.click();
		}
	}

	function resizeBody() {
		if (!bodyInput) return;
		bodyInput.style.height = 'auto';
		bodyInput.style.height = `${bodyInput.scrollHeight}px`;
	}

	function syncMentionPicker() {
		if (!bodyInput) {
			mentionStart = null;
			mentionQuery = '';
			return;
		}
		const state = mentionQueryAt(body, bodyInput.selectionStart ?? body.length);
		if (!state) {
			mentionStart = null;
			mentionQuery = '';
			return;
		}
		mentionStart = state.start;
		mentionQuery = state.query;
	}

	function onBodyInput() {
		resizeBody();
		syncMentionPicker();
	}

	function pickMember(member: MemberInfo) {
		if (mentionStart == null || !bodyInput) return;
		const cursor = bodyInput.selectionStart ?? body.length;
		body = insertMention(body, mentionStart, cursor, member.name);
		const nextPos = mentionStart + member.name.length + 1;
		mentionStart = null;
		mentionQuery = '';
		queueMicrotask(() => {
			bodyInput?.focus();
			bodyInput?.setSelectionRange(nextPos, nextPos);
			resizeBody();
		});
	}

	onMount(async () => {
		if (!circle.canWrite) {
			goto(`/circles/${circle.circleId}`, { replaceState: true });
			return;
		}
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
					const preview =
						m.kind === 'attachment' ? undefined : await getMediaUrl(circle.origin, m.blob_id);
					items.push({
						preview,
						blobId: m.blob_id,
						fileName: m.filename,
						meta: {
							kind: m.kind,
							is_cover: m.is_cover,
							captured_at: m.captured_at,
							geo_lat: m.geo_lat,
							geo_lng: m.geo_lng
						}
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
			const draft = readDraft();
			if (draft) body = draft;
			draftReady = true;
		}
		try {
			members = await fetchMembers(circle.origin, circle.circleId);
		} catch {
			members = [];
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

	function postQueuePayload(trimmed: string): PostQueuePayload {
		return {
			body: trimmed,
			entry_date: entryDate,
			media_meta: picked.map((p) => ({ ...p.meta }))
		};
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

	function removePicked(index: number) {
		const next = picked.filter((_, i) => i !== index);
		if (next.length && !next.some((p) => p.meta.is_cover && isVisual(p.meta))) {
			const firstVisual = next.findIndex((p) => isVisual(p.meta));
			if (firstVisual >= 0) {
				next[firstVisual] = {
					...next[firstVisual],
					meta: { ...next[firstVisual].meta, is_cover: true }
				};
			}
		}
		picked = next;
	}

	async function buildEditMedia(): Promise<MediaSummary[]> {
		const out: MediaSummary[] = [];
		for (const item of picked) {
			let blobId = item.blobId;
			if (!blobId && item.file) {
				blobId = await uploadBlob(circle.origin, item.file);
			}
			if (!blobId) continue;
			out.push({
				blob_id: blobId,
				kind: item.meta.kind as MediaSummary['kind'],
				captured_at: item.meta.captured_at,
				geo_lat: item.meta.geo_lat,
				geo_lng: item.meta.geo_lng,
				is_cover: item.meta.is_cover ?? false
			});
		}
		return out;
	}

	async function onFilesSelected(e: Event) {
		const input = e.target as HTMLInputElement;
		const files = input.files;
		if (!files?.length) return;
		if (filePickLock) {
			input.value = '';
			return;
		}
		filePickLock = true;
		try {
			const compression = await fetchCompression(circle.origin).catch(() => undefined);
			const next = [...picked];
			const list = [...files];
			const videoCount = list.filter(isVideoFile).length;
			compressTotal = videoCount;
			compressIndex = 0;
			keepOpenHint = list.some((file) => isVideoFile(file) && isLargeVideo(file.size));
			compressing = videoCount > 0;
			compressProgress = 0;

			// Потолок вложения сервера: больший файл вернулся бы 413 уже после
			// полной отправки — отсекаем здесь, после сжатия (план 42, MED-5).
			const maxBytes = compression?.attachment_max_bytes ?? 0;
			const tooLarge: string[] = [];
			for (const file of list) {
				const exif = await readExif(file);
				let queueFile: QueueFile;
				if (isImageFile(file)) {
					queueFile = await compressImage(file, compression);
				} else if (isVideoFile(file)) {
					compressIndex += 1;
					compressProgress = 0;
					queueFile = await compressVideo(file, compression, (progress) => {
						compressProgress = progress;
					});
				} else {
					queueFile = await fileToQueueBuffer(file);
				}
				if (maxBytes > 0 && queueFile.size > maxBytes) {
					tooLarge.push(file.name);
					continue;
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
				picked = [...next];
			}
			if (tooLarge.length) {
				error = `Больше ${formatBytes(maxBytes)} — сервер не примет: ${tooLarge.join(', ')}`;
			}
		} finally {
			compressing = false;
			compressProgress = 0;
			keepOpenHint = false;
			filePickLock = false;
			input.value = '';
		}
	}

	async function publish() {
		error = '';
		const trimmed = body.trim();
		if (!trimmed && !picked.length) {
			error = 'Добавьте текст или вложение';
			return;
		}
		if (textByteLength(trimmed) > MAX_TEXT_BYTES) {
			error = 'Текст длиннее 32 КБ';
			return;
		}
		loading = true;
		try {
			if (isEdit && editPostId) {
				const media = await buildEditMedia();
				await savePost(circle.origin, circle.circleId, editPostId, trimmed, entryDate, media);
				goto(feedHref());
				return;
			}

			const files = picked.map((p) => p.file!).filter(Boolean);
			const payload = postQueuePayload(trimmed);

			if (editQueueId) {
				await updateQueuedPost(editQueueId, payload, files);
				goto(feedHref());
				return;
			}

			if (!navigator.onLine) {
				await enqueuePost(circle.origin, circle.circleId, payload, files);
				clearDraft();
				goto(feedHref());
				return;
			}

			const mediaMeta = picked.map((p) => ({ ...p.meta }));
			const media: MediaSummary[] = [];
			for (let i = 0; i < picked.length; i++) {
				const item = picked[i];
				if (!item.file) continue;
				const meta = mediaMeta[i] ?? item.meta;
				const blobId = await uploadBlob(circle.origin, item.file);
				media.push({
					blob_id: blobId,
					kind: meta.kind as MediaSummary['kind'],
					captured_at: meta.captured_at,
					geo_lat: meta.geo_lat,
					geo_lng: meta.geo_lng,
					is_cover: meta.is_cover ?? false
				});
			}

			await createPost(circle.origin, circle.circleId, {
				body: trimmed,
				entry_date: entryDate,
				captured_at: picked[0]?.meta.captured_at,
				media
			});
			clearDraft();
			goto(feedHref(entryDate));
		} catch (err) {
			if (!isEdit && !editQueueId && isTransportError(err)) {
				const files = picked.map((p) => p.file!).filter(Boolean);
				await enqueuePost(circle.origin, circle.circleId, postQueuePayload(trimmed), files);
				clearDraft();
				goto(feedHref());
				return;
			}
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
			oninput={onBodyInput}
			onclick={syncMentionPicker}
			onkeyup={syncMentionPicker}
		/>

		{#if showMentionPicker}
			<MentionPicker>
				{#each mentionCandidates as member, i (member.identity_id)}
					<MemberRow
						initial={circleInitial(member.name)}
						name={member.name}
						color={memberAvatarColor(i)}
						onclick={() => pickMember(member)}
						style={i === 0 ? 'padding:10px 14px' : undefined}
					/>
				{/each}
			</MentionPicker>
			<Hint>
				Список — участники этого круга. Выбрали — в текст встаёт @имя, этому человеку уходит
				пуш. Имя не нажимается: страницы участника нет.
			</Hint>
		{/if}

		<div class="thumbs">
			{#each picked as item, i (i)}
				<MediaTile
					variant="compose"
					src={item.preview}
					kind={item.meta.kind === 'video' ? 'video' : item.meta.kind === 'photo' ? 'photo' : undefined}
					isCover={item.meta.is_cover && isVisual(item.meta)}
					fileName={item.preview ? undefined : (item.file?.name ?? item.fileName)}
					onclick={() => {
						if (isVisual(item.meta)) setCover(i);
					}}
					onremove={() => removePicked(i)}
				/>
			{/each}
			<AddPhotoButton onclick={() => photoInput?.click()} />
		</div>

		{#if compressing}
			<Hint>{compressHint}</Hint>
			{#if keepOpenHint}
				<Hint>Не сворачивайте приложение, пока видео сжимается.</Hint>
			{/if}
		{/if}

		{#if picked.some((p) => isVisual(p.meta))}
			<Hint>Обложка — первая. Нажмите на другую, чтобы лента показывала её.</Hint>
		{/if}

		<div class="date-row">
			<input bind:this={dateInput} type="date" bind:value={entryDate} class="date-pick" tabindex="-1" />
			<SettingsRow
				icon="clock"
				title="Отнести к дате"
				subtitle={entryDateSubtitle}
				style="margin-top:16px;border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
				onclick={openDatePicker}
			/>
		</div>
		{#if isEdit}
			<Hint>
				В ленте запись останется на своём месте. Дата нужна дню — в «Днях» она соберётся с
				остальными за {entryDate ? formatEntryDate(entryDate) : 'этот день'}.
			</Hint>
		{:else}
			<Hint>
				В ленте запись всё равно встанет сегодняшним числом. Дата нужна дню — в «Днях» она
				соберёт её с остальными за {entryDate ? formatEntryDate(entryDate) : 'этот день'}.
			</Hint>
		{/if}

		{#if isEdit && editingPost?.editable_until}
			<SettingsRow
				icon="clock"
				title={editWindowTitle}
				subtitle={windowsDiverged && editingPost
					? editWindowSubtitle(editingPost)
					: undefined}
				chevron={false}
				style="margin-top:18px;border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
			/>
			{#if windowsDiverged}
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
				onclick={() => photoInput?.click()}
			/>
			<IconButton name="file" label="Файл" onclick={() => attachInput?.click()} />
			<span class="who">до 32 КБ · как {circle.identityName}</span>
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
