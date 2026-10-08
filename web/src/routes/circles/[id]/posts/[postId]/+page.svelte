<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import PostByline from '$ui/data/PostByline.svelte';
	import PullRefreshBand from '$ui/data/PullRefresh.svelte';
	import { PullRefresh } from '$lib/gestures/pullRefresh.svelte';
	import ReactionsSheet from '$ui/overlays/ReactionsSheet.svelte';
	import AttachmentList from '$ui/data/AttachmentList.svelte';
	import MentionText from '$ui/data/MentionText.svelte';
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onDestroy, onMount, tick } from 'svelte';
	import Thread from '$ui/data/Thread.svelte';
	import CommentMedia from '$ui/data/CommentMedia.svelte';
	import CommentRow from '$ui/data/CommentRow.svelte';
	import Lightbox from '$ui/overlays/Lightbox.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchMembers, type MemberInfo } from '$lib/circles/settings';
	import { formatClock, formatPostTime, isEditableActive } from '$lib/format/time';
	import { isPostArchiveLocked } from '$lib/journal/archive';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import {
		attachmentMedia,
		authorInitial,
		coverMedia,
		findPost,
		groupReactions,
		ownReaction,
		reactionIconName,
		REACTION_KEYS,
		albumHref,
		commentMediaItems,
		commentPhotoIds,
		photoMedia,
		tileBlobId,
		type CommentMediaItem
	} from '$lib/journal/present';
	import { applyOwnReaction, canReact } from '$lib/journal/reactions';
	import {
		createComment,
		deleteComment,
		editComment,
		fetchCompression,
		removeReaction,
		setReaction
	} from '$lib/journal/posts';
	import type { Comment, FeedPost, MediaSummary } from '$lib/journal/types';
	import { downloadBlob, getMediaUrl } from '$lib/media/objectUrl';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { compressAudio, compressImage, fileToQueueBuffer, isImageFile } from '$lib/media/compress';
	import { audioMime } from '$lib/media/audioTags';
	import type { VoiceTake } from '$lib/media/voiceRecorder.svelte';
	import type { QueueFile, QueueMediaMeta } from '$lib/idb/db';
	import { formatBytes } from '$lib/format/bytes';
	import { numberParam } from '$lib/nav/url';
	import { uuid } from '$lib/uuid';
	import {
		enqueueComment,
		enqueueReaction,
		enqueueReactionRemove,
		listQueuedComments,
		queuedTimeLabel,
		subscribeQueue,
		subscribeQueueProgress,
		type QueuedCommentView
	} from '$lib/queue/queue';
	import { isTransportError } from '$lib/queue/transport';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const postId = $derived($page.params.postId ?? '');

	let post = $state<FeedPost | undefined>();
	let coverUrl = $state('');
	let audioCoverUrls = $state<Record<string, string>>({});
	let postAvatarUrl = $state('');
	let commentAvatarUrls = $state<Record<string, string>>({});
	let loading = $state(true);
	let error = $state('');
	let draft = $state('');
	// Флаги «идёт отправка»: защита от второго нажатия (GUI-8).
	let sending = $state(false);
	let reacting = $state(false);
	let editingCommentId = $state('');
	let editingCommentBody = $state('');
	let activeMemberCount = $state(2);
	let members = $state<MemberInfo[]>([]);
	let queuedComments = $state<QueuedCommentView[]>([]);
	let pickerOpen = $state(false);

	const commentMembers = $derived(members.filter((m) => m.status === 'active'));

	const soloCircle = $derived(activeMemberCount === 1);
	const reactionActor = $derived({
		identityId: circle.identityId,
		identityName: circle.identityName
	});
	const reactionsOpen = $derived($page.url.searchParams.has('reactions'));

	const archiveHint =
		'Эта запись уйдёт с сервера в срок архивации. Новые комментарии и оценки к ней не принимаются.';

	const postLocked = $derived(
		post
			? isPostArchiveLocked(
					Boolean(circle.archiveCycle?.active),
					circle.archiveCycle?.cutoff_date,
					post.created_at
				)
			: false
	);

	// Снимки реплик в очереди ещё на устройстве: адреса живут, пока они в ней.
	let queuedUrls = $state<Record<string, string>>({});
	// Доля ушедших байт у реплики, которая отправляется сейчас.
	let sendProgress = $state<Record<number, number>>({});

	function refreshQueued() {
		void listQueuedComments(circle.origin, circle.circleId, postId).then((items) => {
			// Реплика ушла из очереди — она уже на сервере, перечитываем нить.
			const left = items.length < queuedComments.length;
			const next: Record<string, string> = {};
			for (const item of items) {
				item.media.forEach((m, i) => {
					if (!m.preview) return;
					const key = `${item.id}:${i}`;
					next[key] =
						queuedUrls[key] ??
						URL.createObjectURL(new Blob([m.preview.data], { type: m.preview.type }));
				});
			}
			for (const [key, url] of Object.entries(queuedUrls)) {
				if (!next[key]) URL.revokeObjectURL(url);
			}
			queuedUrls = next;
			queuedComments = items;
			if (left) void load();
		});
	}

	function queuedMediaItems(item: QueuedCommentView): CommentMediaItem[] {
		return item.media.map((m, i) => {
			const key = `${item.id}:${i}`;
			if (m.kind === 'photo') return { key, kind: 'photo', url: queuedUrls[key] };
			if (m.voice) return { key, kind: 'voice', peaks: m.peaks, durationMs: m.duration_ms };
			return { key, kind: 'file', name: m.name, size: m.size };
		});
	}

	// Вложения комментария (4.28): выбранное ждёт отправки полоской над полем.
	const MAX_COMMENT_MEDIA = 10;
	interface PendingMedia {
		key: string;
		file: QueueFile;
		meta: QueueMediaMeta;
		url?: string;
	}
	// $state.raw: файлы уходят в IndexedDB, а реактивную обёртку она не клонирует.
	let pending = $state.raw<PendingMedia[]>([]);
	let preparing = $state('');
	// Адреса снимков из комментариев по blob id.
	let mediaUrls = $state<Record<string, string>>({});

	function takeRoom(files: File[]): File[] {
		const room = MAX_COMMENT_MEDIA - pending.length;
		if (files.length > room) error = `В комментарии не больше ${MAX_COMMENT_MEDIA} вложений`;
		return files.slice(0, Math.max(0, room));
	}

	async function addPhotos(files: File[]) {
		error = '';
		const picked = takeRoom(files.filter(isImageFile));
		if (!picked.length) return;
		// Сжатие снимка — секунда-другая: говорим, что выбор принят.
		preparing = 'Готовим фото…';
		try {
			const compression = await fetchCompression(circle.origin).catch(() => undefined);
			for (const file of picked) {
				const image = await compressImage(file, compression).catch(() => fileToQueueBuffer(file));
				pending = [
					...pending,
					{
						key: uuid(),
						file: image,
						meta: { kind: 'photo' },
						url: URL.createObjectURL(new Blob([image.data], { type: image.type }))
					}
				];
			}
		} finally {
			preparing = '';
		}
	}

	async function addFiles(files: File[]) {
		error = '';
		const picked = takeRoom(files);
		if (!picked.length) return;
		const compression = await fetchCompression(circle.origin).catch(() => undefined);
		const maxBytes = compression?.attachment_max_bytes ?? 0;
		for (const file of picked) {
			if (maxBytes > 0 && file.size > maxBytes) {
				error = `Файл больше ${formatBytes(maxBytes)} — сервер не примет`;
				continue;
			}
			pending = [
				...pending,
				{ key: uuid(), file: await fileToQueueBuffer(file), meta: { kind: 'attachment' } }
			];
		}
	}

	function dropPending(list: PendingMedia[]) {
		for (const p of list) if (p.url) URL.revokeObjectURL(p.url);
	}

	function removePending(index: number) {
		dropPending(pending.slice(index, index + 1));
		pending = pending.filter((_, i) => i !== index);
	}

	// Реплика с вложением всегда идёт через очередь, как запись с фото: файл
	// догружается после обрыва, без сети ждёт её, второй раз не создаётся.
	async function enqueueWithMedia(text: string, items: PendingMedia[]) {
		await enqueueComment(
			circle.origin,
			circle.circleId,
			{ post_id: postId, body: text, media_meta: items.map((p) => p.meta) },
			items.map((p) => p.file)
		);
		refreshQueued();
	}

	async function sendVoice(take: VoiceTake) {
		if (postLocked || preparing) return;
		preparing = 'Готовим голосовое…';
		error = '';
		try {
			const compression = await fetchCompression(circle.origin).catch(() => undefined);
			// Как голосовое записью (4.24): 64 кбит/с, перекодируем всегда.
			const { fallbackReason: _reason, ...audio } = await compressAudio(
				take.file,
				compression,
				undefined,
				{ bitrateKbps: 64, force: true }
			);
			await enqueueWithMedia('', [
				{
					key: uuid(),
					file: { ...audio, type: audioMime(audio.name, audio.type) },
					meta: {
						kind: 'attachment',
						voice: true,
						audio_duration_ms: Math.round(take.durationMs),
						audio_peaks: take.peaks
					}
				}
			]);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			preparing = '';
		}
	}

	function downloadFile(item: CommentMediaItem) {
		if (item.blobId) void downloadBlob(circle.origin, item.blobId, item.name || 'файл');
	}

	// Снимок комментария во весь экран (4.29): листаем в пределах реплики.
	// Состояние — в адресе: «Назад» закрывает, а превью в ленте (4.30)
	// открывает сразу нужный снимок.
	const lbComment = $derived($page.url.searchParams.get('cphoto') ?? '');
	const lbPhotos = $derived(
		lbComment ? commentPhotoIds(post?.comments?.find((c) => c.id === lbComment)?.media) : []
	);
	const lbIndex = $derived(
		Math.min(Math.max(numberParam($page.url, 'lb', 0), 0), Math.max(lbPhotos.length - 1, 0))
	);
	// Экран пересоздаётся при смене адреса, поэтому «открыли отсюда» — тоже
	// в адресе: закрытие тогда оставляет в обсуждении, а не уводит туда,
	// откуда пришли.
	const lbOpenedHere = $derived($page.url.searchParams.has('here'));

	function commentPhotoUrl(commentId: string, index: number, here: boolean): string {
		const params = new URLSearchParams({ cphoto: commentId, lb: String(index) });
		if (here) params.set('here', '1');
		return `${$page.url.pathname}?${params}`;
	}

	function openCommentPhoto(commentId: string, index: number) {
		goto(commentPhotoUrl(commentId, index, true));
	}

	function showCommentPhoto(index: number) {
		goto(commentPhotoUrl(lbComment, index, lbOpenedHere), { replaceState: true });
	}

	function closeCommentPhoto() {
		if (lbOpenedHere) {
			// Не шаг истории: системную «Назад» экран забирает себе (C13).
			goto($page.url.pathname, { replaceState: true });
			return;
		}
		// Открыли из ленты или по ссылке — возвращаемся туда, откуда пришли.
		goBack();
	}

	onDestroy(() => {
		dropPending(pending);
		for (const url of Object.values(queuedUrls)) URL.revokeObjectURL(url);
	});

	// Обновление жестом (3.5), как в ленте: тянешь экран от верха.
	let listEl: HTMLDivElement | undefined = $state();
	const ptr = new PullRefresh(() => listEl?.scrollTop ?? 0, () => load());
	onDestroy(() => ptr.destroy());

	onMount(() => {
		void fetchMembers(circle.origin, circle.circleId)
			.then((list) => {
				members = list;
				activeMemberCount = list.filter((m) => m.status === 'active').length;
			})
			.catch(() => {
				members = [];
				activeMemberCount = 2;
			});
		void load();
		refreshQueued();
		const unsubQueue = subscribeQueue(refreshQueued);
		const unsubProgress = subscribeQueueProgress((id, fraction) => {
			const next = { ...sendProgress };
			if (fraction === undefined) delete next[id];
			else next[id] = fraction;
			sendProgress = next;
		});
		const unsubSync = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['feed'],
			refetch: load
		});
		return () => {
			unsubQueue();
			unsubProgress();
			unsubSync();
		};
	});

	async function load() {
		try {
			const snap = await loadFeed(circle.origin, circle.circleId);
			post = findPost(snap.posts, postId);
			const cover = post ? coverMedia(post.media) : undefined;
			if (cover) {
				// Ролик без кадра — плитка со значком, а не ошибка всей записи.
				coverUrl = await getMediaUrl(circle.origin, tileBlobId(cover)).catch(() => '');
			} else {
				coverUrl = '';
			}
			const covers: Record<string, string> = {};
			for (const att of post?.media ?? []) {
				const id = att.audio_cover_blob_id;
				if (!id || covers[id]) continue;
				try {
					covers[id] = await getMediaUrl(circle.origin, id);
				} catch {
					/* плитка останется пустой */
				}
			}
			audioCoverUrls = covers;
			if (post?.author_avatar_blob_id) {
				try {
					postAvatarUrl = await getMediaUrl(circle.origin, post.author_avatar_blob_id);
				} catch {
					postAvatarUrl = '';
				}
			} else {
				postAvatarUrl = '';
			}
			const avatars: Record<string, string> = {};
			for (const comment of post?.comments ?? []) {
				const blob = comment.author_avatar_blob_id;
				if (blob && !avatars[comment.identity_id]) {
					try {
						avatars[comment.identity_id] = await getMediaUrl(circle.origin, blob);
					} catch {
						/* skip */
					}
				}
			}
			commentAvatarUrls = avatars;
			// Снимки из комментариев — пачками поверх, нить их не ждёт.
			const photoIds = new Set<string>();
			for (const comment of post?.comments ?? []) {
				for (const id of commentPhotoIds(comment.media)) {
					if (!mediaUrls[id]) photoIds.add(id);
				}
			}
			void resolveMediaUrls(circle.origin, [...photoIds], (blobId, url) => {
				mediaUrls = { ...mediaUrls, [blobId]: url };
			});
		} catch {
			error = 'Не удалось загрузить запись';
		} finally {
			loading = false;
		}
		void showLinkedComment();
	}

	// Из «Откликов» (3.13) и из поиска найденное открывается у себя:
	// комментарий — в обсуждении, файл или звук — строкой, запись — целиком.
	// Экран встаёт на него и на миг подсвечивает. Один раз за вход.
	let linkedShown = false;
	async function showLinkedComment() {
		const params = $page.url.searchParams;
		const comment = params.get('comment');
		const media = params.get('media');
		const found = params.has('found');
		if ((!comment && !media && !found) || linkedShown) return;
		linkedShown = true;
		await tick();
		const selector = comment
			? `.cmt.c-${CSS.escape(comment)}`
			: media
				? `.att.m-${CSS.escape(media)}`
				: '.post';
		const el = document.querySelector<HTMLElement>(selector);
		if (!el) return;
		el.scrollIntoView({ block: 'center' });
		el.classList.add('linked');
		setTimeout(() => el.classList.remove('linked'), 1600);
	}

	// Родитель записи — экран круга, где её открыли: вкладка, день, поиск
	// (план 46, C13). Пришли иначе (пуш, ссылка, своя же правка) — лента.
	let backTo = '';
	afterNavigate(({ from }) => {
		if (backTo || !from) return;
		const base = `/circles/${circle.circleId}`;
		const rest = from.url.pathname.startsWith(base) ? from.url.pathname.slice(base.length) : null;
		if (rest !== null && /^(|\/days(\/\d{4}-\d{2}-\d{2})?|\/grid|\/map|\/responses|\/search)$/.test(rest)) {
			backTo = from.url.pathname + from.url.search;
		}
	});

	function goBack() {
		goUp(backTo || `/circles/${circle.circleId}`);
	}

	function openEdit() {
		goto(`/circles/${circle.circleId}/compose?post=${postId}`);
	}

	function openAlbum() {
		goto(albumHref(circle.circleId, postId, photoMedia(post?.media).length));
	}

	function canEditComment(comment: Comment): boolean {
		return (
			circle.canWrite &&
			comment.identity_id === circle.identityId &&
			isEditableActive(comment.editable_until)
		);
	}

	function canEditPost(currentPost: FeedPost): boolean {
		return (
			circle.canWrite &&
			currentPost.identity_id === circle.identityId &&
			isEditableActive(currentPost.editable_until)
		);
	}

	function startEditComment(comment: Comment) {
		editingCommentId = comment.id;
		editingCommentBody = comment.body;
	}

	function cancelEditComment() {
		editingCommentId = '';
		editingCommentBody = '';
	}

	async function saveCommentEdit(commentId: string, hasMedia: boolean) {
		const text = editingCommentBody.trim();
		// Слова можно стереть, только если у реплики остаются вложения (4.7).
		if (!text && !hasMedia) return;
		error = '';
		try {
			await editComment(circle.origin, circle.circleId, postId, commentId, text);
			cancelEditComment();
			await load();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function removeComment(commentId: string) {
		error = '';
		try {
			await deleteComment(circle.origin, circle.circleId, postId, commentId);
			await load();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function sendComment() {
		// Два быстрых нажатия — два комментария (GUI-8).
		if (postLocked || sending) return;
		const text = draft.trim();
		if (!text && !pending.length) return;
		sending = true;
		error = '';
		// Ключ один на все попытки: оборванный ответ не создаст вторую реплику.
		const clientId = uuid();
		if (pending.length) {
			// Своя ветка: при сбое вложения остаются над полем, а не уходит
			// один текст без них.
			const items = pending;
			try {
				await enqueueWithMedia(text, items);
				pending = [];
				dropPending(items);
				draft = '';
			} catch (err) {
				error = authErrorHint(err);
			} finally {
				sending = false;
			}
			return;
		}
		try {
			if (navigator.onLine) {
				await createComment(circle.origin, circle.circleId, postId, text, clientId);
				draft = '';
				await load();
			} else {
				await enqueueComment(circle.origin, circle.circleId, { post_id: postId, body: text });
				draft = '';
				refreshQueued();
			}
		} catch (err) {
			if (isTransportError(err)) {
				await enqueueComment(circle.origin, circle.circleId, { post_id: postId, body: text });
				draft = '';
				refreshQueued();
				return;
			}
			error = authErrorHint(err);
		} finally {
			sending = false;
		}
	}

	function commentAvatarSrc(comment: Comment): string | undefined {
		if (comment.identity_id === circle.identityId && circle.avatarUrl) return circle.avatarUrl;
		return commentAvatarUrls[comment.identity_id];
	}

	function openReactions() {
		goto(`${$page.url.pathname}?reactions=1`);
	}

	function closeReactions() {
		goto($page.url.pathname);
	}

	function showReactionPlus(currentPost: FeedPost): boolean {
		return canReact(currentPost.reactions, reactionActor, {
			canWrite: circle.canWrite,
			solo: soloCircle,
			locked: postLocked
		});
	}

	function applyReaction(emoji: string | null) {
		if (!post) return;
		post = {
			...post,
			reactions: applyOwnReaction(post.reactions, post.id, emoji, reactionActor)
		};
	}

	async function pickReaction(currentPost: FeedPost, emoji: string) {
		if (postLocked || reacting) return;
		reacting = true;
		const mine = ownReaction(currentPost.reactions, circle.identityId);
		const removing = mine?.emoji === emoji;
		error = '';
		try {
			if (navigator.onLine) {
				if (removing) {
					await removeReaction(circle.origin, circle.circleId, currentPost.id);
				} else {
					await setReaction(circle.origin, circle.circleId, currentPost.id, emoji);
				}
				pickerOpen = false;
				await load();
			} else {
				if (removing) {
					await enqueueReactionRemove(circle.origin, circle.circleId, currentPost.id);
					applyReaction(null);
				} else {
					await enqueueReaction(circle.origin, circle.circleId, {
						post_id: currentPost.id,
						emoji
					});
					applyReaction(emoji);
				}
				pickerOpen = false;
			}
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			reacting = false;
		}
	}
</script>

<CircleLayout
	app
	color={circle.color}
	title="Обсуждение"
	identity={circle.identityName}
	avatar={circle.identityInitial}
	avatarSrc={circle.avatarUrl}
	tabs={false}
	identitySettingsLink={circle.canWrite}
	commentPlaceholder="Написать комментарий…"
	commentMembers={commentMembers}
	commentBusy={sending}
	bind:commentDraft={draft}
	commentBar={circle.canWrite && !postLocked}
	commentStatus={preparing}
	commentPhotoAccept="image/*"
	commentPending={pending.map((p) => ({ key: p.key, url: p.url, name: p.file.name }))}
	onCommentRemovePending={removePending}
	onCommentPhotos={(files) => void addPhotos(files)}
	onCommentFiles={(files) => void addFiles(files)}
	onCommentVoice={(take) => void sendVoice(take)}
	onback={goBack}
	onCommentSend={circle.canWrite && !postLocked ? sendComment : undefined}
>
	<PullRefreshBand pull={ptr.state} />
	<div
		class="feed"
		role="feed"
		aria-label="Запись"
		bind:this={listEl}
		ontouchstart={ptr.start}
		ontouchmove={ptr.move}
		ontouchend={ptr.end}
	>
	{#if loading}
		<Loading />
	{:else if !post}
		<Hint class="gutter-24">{error || 'Запись не найдена'}</Hint>
	{:else}
		{@const currentPost = post}
		{@const cover = coverMedia(currentPost.media)}
		{#snippet postHeaderRight()}
			<div class="gap-10 ml-auto flex-mid">
				{#if canEditPost(currentPost)}
					<IconButton name="edit" label="Править" size="sm" onclick={openEdit} />
				{/if}
				{#if cover && (coverUrl || cover.kind === 'video')}
					<MediaTile
						variant="headerMini"
						src={coverUrl || undefined}
						kind={cover.kind === 'video' ? 'video' : 'photo'}
						aria-label="Открыть альбом"
						onclick={openAlbum}
					/>
				{/if}
			</div>
		{/snippet}
		{#snippet postText()}
			<MentionText body={currentPost.body} />
		{/snippet}
		{#snippet postMedia()}
			<AttachmentList
				items={attachmentMedia(currentPost.media)}
				origin={circle.origin}
				circleId={circle.circleId}
				circleName={circle.name}
				color={circle.colorHex}
				postId={currentPost.id}
				coverUrls={audioCoverUrls}
			/>
		{/snippet}
		<PostCard
			class="mt-8"
			headerRight={postHeaderRight}
			text={currentPost.body ? postText : undefined}
			media={attachmentMedia(currentPost.media).length ? postMedia : undefined}
		>
			{#snippet author()}
				<PostByline
					initial={authorInitial(currentPost.author_name)}
					color={circle.colorHex}
					src={currentPost.identity_id === circle.identityId && circle.avatarUrl
						? circle.avatarUrl
						: postAvatarUrl || undefined}
					name={currentPost.author_name}
					time={formatPostTime(currentPost.created_at, currentPost.entry_date)}
				/>
			{/snippet}
			{#snippet reactions()}
				{#if !soloCircle}
					<ReactionBar
						groups={groupReactions(currentPost.reactions).map((g) => ({
							icon: reactionIconName(g.emoji),
							names: g.names
						}))}
						keys={REACTION_KEYS}
						showAdd={showReactionPlus(currentPost) && !pickerOpen}
						pickerOpen={pickerOpen}
						selectedKey={ownReaction(currentPost.reactions, circle.identityId)?.emoji ?? ''}
						onopenList={openReactions}
						onadd={() => {
							pickerOpen = true;
						}}
						onpick={(key) => void pickReaction(currentPost, key)}
					/>
				{/if}
			{/snippet}
		</PostCard>
		{#if postLocked}
			<Hint class="m-0-16-12">{archiveHint}</Hint>
		{/if}

		<Thread>
			{#each currentPost.comments ?? [] as comment (comment.id)}
				<CommentRow
					class="c-{comment.id}"
					initial={authorInitial(comment.author_name)}
					name={comment.author_name}
					color={circle.colorHex}
					src={commentAvatarSrc(comment)}
					onedit={canEditComment(comment) && editingCommentId !== comment.id
						? () => startEditComment(comment)
						: undefined}
					ondelete={canEditComment(comment) && editingCommentId !== comment.id
						? () => removeComment(comment.id)
						: undefined}
				>
					{#snippet time()}{formatClock(comment.created_at)}{/snippet}
					{#snippet children()}
						{#if editingCommentId === comment.id}
							<TextArea variant="commentEdit" bind:value={editingCommentBody} />
							<div class="rowin">
								<Button class="grow-flat"
									variant="colored"
									keepFocus
									onclick={() => saveCommentEdit(comment.id, Boolean(comment.media?.length))}
								>
									Сохранить
								</Button>
								<Button class="grow-flat" variant="ghost" keepFocus onclick={cancelEditComment}>
									Отмена
								</Button>
							</div>
							<CommentMedia items={commentMediaItems(comment.media, mediaUrls)} inert />
						{:else}
							{#if comment.body}
								<MentionText body={comment.body} />
							{/if}
							<CommentMedia
								items={commentMediaItems(comment.media, mediaUrls)}
								origin={circle.origin}
								audio={{
									circleId: circle.circleId,
									circleName: circle.name,
									color: circle.colorHex,
									postId: currentPost.id
								}}
								onphoto={(i) => openCommentPhoto(comment.id, i)}
								onfile={downloadFile}
							/>
						{/if}
					{/snippet}
				</CommentRow>
			{/each}
			{#each queuedComments as item (item.id)}
				<CommentRow
					queued
					initial={circle.identityInitial}
					name={circle.identityName}
					color={circle.colorHex}
					src={circle.avatarUrl}
				>
					{#snippet time()}
						<span class="flex-mid gap-5">
							<Icon name="clock" size="xs" />{queuedTimeLabel(item.state, sendProgress[item.id])}
						</span>
					{/snippet}
					{#snippet children()}
						{#if item.body}
							<MentionText body={item.body} />
						{/if}
						<CommentMedia items={queuedMediaItems(item)} inert />
						{#if item.state === 'failed' && item.error}
							<Hint class="mt-8">{item.error}</Hint>
						{/if}
					{/snippet}
				</CommentRow>
			{/each}
		</Thread>

		{#if error}
			<Hint class="m-0-16-16">{error}</Hint>
		{/if}
	{/if}
	</div>

	{#if reactionsOpen && post}
		<ReactionsSheet reactions={post.reactions ?? []} color={circle.colorHex} ondismiss={closeReactions} />
	{/if}
</CircleLayout>

{#if lbComment && lbPhotos.length}
	{@const blobId = lbPhotos[lbIndex]}
	<Lightbox
		fixed
		counter="{lbIndex + 1} из {lbPhotos.length}"
		dotCount={lbPhotos.length}
		dotIndex={lbIndex}
		onDotSelect={showCommentPhoto}
		onclose={closeCommentPhoto}
		ondownload={() => void downloadBlob(circle.origin, blobId, 'photo.jpg')}
		onprev={lbIndex > 0 ? () => showCommentPhoto(lbIndex - 1) : undefined}
		onnext={lbIndex < lbPhotos.length - 1 ? () => showCommentPhoto(lbIndex + 1) : undefined}
	>
		{#snippet media()}
			{#if mediaUrls[blobId]}
				<img src={mediaUrls[blobId]} alt="" />
			{/if}
		{/snippet}
	</Lightbox>
{/if}

