<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import PostByline from '$ui/data/PostByline.svelte';
	import FeedEnd from '$ui/data/FeedEnd.svelte';
	import PullRefreshBand from '$ui/data/PullRefresh.svelte';
	import { PullRefresh } from '$lib/gestures/pullRefresh.svelte';
	import EmptyState from '$ui/data/EmptyState.svelte';
	import ReactionsSheet from '$ui/overlays/ReactionsSheet.svelte';
	import AttachmentList from '$ui/data/AttachmentList.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import MentionText from '$ui/data/MentionText.svelte';
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onDestroy, onMount, tick } from 'svelte';
	import ArchiveBanner from '$ui/data/ArchiveBanner.svelte';
	import EntryDateMark from '$ui/data/EntryDateMark.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import EventDivider from '$ui/data/EventDivider.svelte';
	import FeedDayPromptCard from '$ui/data/FeedDayPromptCard.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import CommentPreview from '$ui/data/CommentPreview.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { isAccessError } from '$lib/api/client';
	import { formatBytes } from '$lib/format/bytes';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { WORD, plural } from '$lib/format/plural';
	import { formatDeadline, formatEntryDate, formatPostTime } from '$lib/format/time';
	import { isPostArchiveLocked } from '$lib/journal/archive';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import { fetchOlderPage, mergePages, refetchOlder } from '$lib/journal/pages';
	import { applyOwnReaction, canReact } from '$lib/journal/reactions';
	import { fetchMembers, type MemberInfo } from '$lib/circles/settings';
	import {
		isAttachedToOtherDay,
		attachmentMedia,
		authorInitial,
		commentPreview,
		coverMedia,
		groupReactions,
		locationLabel,
		mediaCount,
		maxReadSeq,
		ownReaction,
		reactionIconName,
		REACTION_KEYS,
		serviceEventsBetween,
		serviceEventsAboveNewest,
		unreadDividerIndex,
		albumHref,
		photoMedia
	} from '$lib/journal/present';
	import { advanceReadCursor } from '$lib/journal/read-cursor';
	import {
		createPost,
		downloadArchive,
		fetchCompression,
		removeReaction,
		setReaction
	} from '$lib/journal/posts';
	import { compressAudio, compressVideo } from '$lib/media/compress';
	import { audioMime } from '$lib/media/audioTags';
	import type { VoiceTake } from '$lib/media/voiceRecorder.svelte';
	import type { QueueMediaMeta } from '$lib/idb/db';
	import VideoRecorder from '$ui/overlays/VideoRecorder.svelte';
	import { loadDays } from '$lib/journal/days';
	import type { FeedEvent, FeedPost, MediaSummary } from '$lib/journal/types';
	import {
		bumpDayPromptCount,
		getDayPromptShowCount,
		isDayPromptDismissed
	} from '$lib/idb/db';
	import {
		enqueuePost,
		enqueueReaction,
		enqueueReactionRemove,
		listQueuedPosts,
		removeQueueItem,
		subscribeQueue,
		type QueuedPostView
	} from '$lib/queue/queue';
	import { isTransportError } from '$lib/queue/transport';
	import { registerRefetch } from '$lib/sync/sync';
	import { authErrorHint } from '$lib/auth/auth';
	import { handComposePhotos } from '$lib/journal/compose-handoff';
	import { holdKeyboard } from '$lib/navigation/keyboard';
	import { applyFeedSpot, feedSpotKey, saveFeedSpot, takeFeedSpot } from '$lib/journal/feed-scroll';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	// Флаги «идёт отправка»: защита от второго нажатия (GUI-8).
	let sending = $state(false);
	let reacting = $state(false);
	// Подтверждение удаления неотправленной записи (STB-6).
	let queueToRemove = $state<number | null>(null);

	let posts = $state<FeedPost[]>([]);
	// Догруженные старшие порции (C18): снимок — самые новые записи, остальное
	// приходит при прокрутке к концу ленты.
	let olderPosts = $state<FeedPost[]>([]);
	let nextBefore = $state<string | undefined>(undefined);
	let loadingOlder = $state(false);
	let feedEvents = $state<FeedEvent[]>([]);
	let visibleFrom = $state<string | null>(null);
	let circleStartedAt = $state('');
	let members = $state<MemberInfo[]>([]);
	let activeMemberCount = $state(2);
	const commentMembers = $derived(members.filter((m) => m.status === 'active'));
	let queuedPosts = $state<QueuedPostView[]>([]);
	let loading = $state(true);
	let error = $state('');
	let joinAvatarHint = $state('');
	let dividerAt = $state<number | null>(null);
	let fixedLastRead = $state(0);
	let feedEl: HTMLDivElement | undefined = $state();
	// Автомат жеста живёт в $lib и покрыт тестами (GUI-5).
	// Обновление жестом (3.5): тянешь ленту от верха — она перечитывается.
	const ptr = new PullRefresh(() => feedEl?.scrollTop ?? 0, () => loadFeedData());
	let commentDraft = $state('');
	let dayPromptDate = $state('');
	let pickerPostId = $state('');

	const reactionsPostId = $derived($page.url.searchParams.get('reactions'));
	const reactionsPost = $derived(
		reactionsPostId ? posts.find((p) => p.id === reactionsPostId) : undefined
	);

	let mediaUrls = $state<Record<string, string>>({});
	let authorAvatarUrls = $state<Record<string, string>>({});

	function authorAvatarSrc(post: FeedPost): string | undefined {
		if (post.identity_id === circle.identityId && circle.avatarUrl) return circle.avatarUrl;
		return authorAvatarUrls[post.identity_id];
	}

	// Лента не ждёт картинок: до двух тысяч записей давали столько же
	// последовательных await, и экран стоял в «загрузке», пока не доедет
	// последняя обложка. Пачки — общий хелпер $lib/media/batch (REF-6).
	// Ход скачивания обложек и неудачи — плитка говорит, что происходит.
	let mediaProgress = $state<Record<string, { received: number; total: number }>>({});
	let mediaFailed = $state<Record<string, boolean>>({});

	async function resolveFeedMedia(feedPosts: FeedPost[]) {
		const avatarJobs = new Map<string, string>();
		const coverJobs = new Map<string, string>();
		for (const post of feedPosts) {
			const avatarBlob = post.author_avatar_blob_id;
			if (avatarBlob && !authorAvatarUrls[post.identity_id]) {
				avatarJobs.set(post.identity_id, avatarBlob);
			}
			const cover = coverMedia(post.media);
			if (cover && !mediaUrls[cover.blob_id]) {
				coverJobs.set(cover.blob_id, cover.blob_id);
			}
			for (const att of attachmentMedia(post.media)) {
				if (att.audio_cover_blob_id && !mediaUrls[att.audio_cover_blob_id]) {
					coverJobs.set(att.audio_cover_blob_id, att.audio_cover_blob_id);
				}
			}
		}

		// Аватар лица и обложка записи — разные ключи при одном блобе, поэтому
		// два прохода: сперва лица, потом обложки.
		const identitiesByBlob = new Map<string, string[]>();
		for (const [identityId, blobId] of avatarJobs) {
			// Один блоб может быть аватаром нескольких лиц — тогда адрес
			// достаётся всем им, а не последнему в списке.
			identitiesByBlob.set(blobId, [...(identitiesByBlob.get(blobId) ?? []), identityId]);
		}
		await resolveMediaUrls(circle.origin, [...identitiesByBlob.keys()], (blobId, url) => {
			for (const identityId of identitiesByBlob.get(blobId) ?? []) {
				authorAvatarUrls = { ...authorAvatarUrls, [identityId]: url };
			}
		});
		await resolveMediaUrls(
			circle.origin,
			[...coverJobs.keys()],
			(blobId, url) => {
				mediaUrls = { ...mediaUrls, [blobId]: url };
			},
			undefined,
			{
				onProgress: (blobId, received, total) => {
					mediaProgress = { ...mediaProgress, [blobId]: { received, total } };
				},
				onFail: (blobId) => {
					mediaFailed = { ...mediaFailed, [blobId]: true };
				}
			}
		);
	}

	const soloCircle = $derived(activeMemberCount === 1);
	const reactionActor = $derived({
		identityId: circle.identityId,
		identityName: circle.identityName
	});
	const showVisibilityCutoff = $derived(Boolean(visibleFrom));

	// Вернулись с записи или альбома — лента встаёт туда, откуда ушли.
	// Ждём и переход (откуда пришли), и первую загрузку: порядок у них любой.
	let cameFromPost: boolean | undefined;
	let feedLoaded = false;
	let spotDone = false;

	afterNavigate(({ from }) => {
		if (cameFromPost !== undefined) return;
		cameFromPost = Boolean(from?.url.pathname.startsWith(`/circles/${circle.circleId}/posts/`));
		void restoreSpot();
	});

	async function restoreSpot() {
		if (spotDone || cameFromPost === undefined || !feedLoaded) return;
		spotDone = true;
		const spot = takeFeedSpot(feedSpotKey(circle.origin, circle.circleId));
		if (!spot || !cameFromPost) return;
		await tick();
		if (feedEl) applyFeedSpot(feedEl, posts.map((p) => p.id), spot);
	}

	function leaveToPost(path: string) {
		saveFeedSpot(feedSpotKey(circle.origin, circle.circleId), feedEl, posts.map((p) => p.id));
		goto(path);
	}

	async function loadFeedData() {
		error = '';
		try {
			const snap = await loadFeed(circle.origin, circle.circleId);
			nextBefore = snap.next_before;
			if (olderPosts.length) {
				// Граница первой порции сдвинулась — перечитываем догруженное от неё.
				const again = await refetchOlder(loadOlderFeed, snap.next_before, olderPosts.length).catch(
					() => ({ items: olderPosts, next: nextBefore })
				);
				olderPosts = again.items;
				nextBefore = again.next;
			}
			posts = mergePages(snap.posts, olderPosts, (p) => p.id);
			feedEvents = snap.events ?? [];
			visibleFrom = snap.visible_from ?? null;
			circleStartedAt = snap.circle_started_at ?? '';
			dividerAt = unreadDividerIndex(posts, fixedLastRead);
			// Не ждём: записи уже есть, картинки доедут пачками поверх.
			void resolveFeedMedia(posts);
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить ленту';
		} finally {
			loading = false;
		}
		feedLoaded = true;
		await restoreSpot();
	}

	async function loadOlderFeed(before: string) {
		const page = await fetchOlderPage<{ posts: FeedPost[] }>(
			circle.origin,
			`/circles/${circle.circleId}/feed`,
			before
		);
		return { items: page.posts, next: page.next_before };
	}

	async function loadOlder() {
		if (!nextBefore || loadingOlder) return;
		loadingOlder = true;
		try {
			const page = await loadOlderFeed(nextBefore);
			olderPosts = [...olderPosts, ...page.items];
			nextBefore = page.next;
			posts = mergePages(posts, page.items, (p) => p.id);
			void resolveFeedMedia(page.items);
		} catch {
			// Без сети старшее не догрузится — лента остаётся как есть.
		} finally {
			loadingOlder = false;
		}
	}

	// До конца ленты — полтора экрана: догружаем заранее, чтобы не упираться.
	function onFeedScroll() {
		const el = feedEl;
		if (!el || !nextBefore) return;
		if (el.scrollTop + el.clientHeight * 2.5 >= el.scrollHeight) void loadOlder();
	}

	function refreshQueued() {
		void listQueuedPosts(circle.origin, circle.circleId).then((items) => {
			queuedPosts = items;
		});
	}

	onMount(() => {
		if ($page.url.searchParams.get('joinAvatar') === 'fail') {
			joinAvatarHint = 'Фото не загрузилось — поставьте в профиле';
			void dropSearchParam('joinAvatar');
		}
		fixedLastRead = circle.lastReadSeq;
		const dayPromptParam = $page.url.searchParams.get('dayPrompt');
		void loadFeedData().then(() => {
			if (!dayPromptParam || !/^\d{4}-\d{2}-\d{2}$/.test(dayPromptParam)) return;
			void dropSearchParam('dayPrompt');
			void checkDayPrompt(dayPromptParam);
		});
		void fetchMembers(circle.origin, circle.circleId)
			.then((list) => {
				members = list;
				activeMemberCount = list.filter((m) => m.status === 'active').length;
			})
			.catch(() => {
				activeMemberCount = 2;
			});
		refreshQueued();
		const unsubQueue = subscribeQueue(refreshQueued);
		const unsubSync = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['feed'],
			refetch: loadFeedData
		});
		return () => {
			unsubQueue();
			unsubSync();
		};
	});

	onDestroy(() => {
		ptr.destroy();
		const seq = maxReadSeq(posts);
		if (seq > 0) {
			void advanceReadCursor(circle.origin, circle.circleId, seq);
		}
	});

	function goBack() {
		goUp('/circles');
	}

	function openSearch() {
		goto(`/circles/${circle.circleId}/search`);
	}

	async function checkDayPrompt(entryDate: string) {
		if (await isDayPromptDismissed(circle.origin, circle.circleId)) return;
		if ((await getDayPromptShowCount(circle.origin, circle.circleId)) >= 3) return;
		try {
			const days = await loadDays(circle.origin, circle.circleId);
			const day = days.days.find((d) => d.entry_date === entryDate);
			if (day?.post_count === 1) dayPromptDate = entryDate;
		} catch {
			/* skip */
		}
	}

	async function sendFromBar() {
		const text = commentDraft.trim();
		// Два быстрых нажатия — две записи: кнопка не блокируется сама (GUI-8).
		if (!text || sending) return;
		sending = true;
		error = '';
		const entryDate = todayEntryDate();
		try {
			if (navigator.onLine) {
				await createPost(circle.origin, circle.circleId, {
					body: text,
					entry_date: entryDate,
					media: []
				});
			} else {
				await enqueuePost(
					circle.origin,
					circle.circleId,
					{ body: text, entry_date: entryDate },
					[]
				);
			}
			commentDraft = '';
			await loadFeedData();
			if (navigator.onLine) await checkDayPrompt(entryDate);
		} catch (err) {
			if (isTransportError(err)) {
				await enqueuePost(
					circle.origin,
					circle.circleId,
					{ body: text, entry_date: entryDate },
					[]
				);
				commentDraft = '';
				await loadFeedData();
				return;
			}
			error = authErrorHint(err);
		} finally {
			sending = false;
		}
	}

	// Удаление неотправленной записи спрашивают: она нигде больше не
	// сохранена, а иконка «три точки» обещала меню (STB-6).
	async function confirmQueueRemove() {
		const id = queueToRemove;
		queueToRemove = null;
		if (!id) return;
		await removeQueueItem(id);
	}

	function openComposeFromBar() {
		sessionStorage.setItem(`wynd.compose.${circle.circleId}`, commentDraft);
		holdKeyboard();
		goto(`/circles/${circle.circleId}/compose`);
	}

	// Голосовое и видео из полосы ввода (C14, 4.24, 4.26): новая запись журнала
	// без текста — «Запись» стоит только у пустого поля. Через очередь — без
	// сети тоже уйдёт.
	let videoOpen = $state(false);
	let preparing = $state('');

	async function enqueueRecorded(
		file: { name: string; type: string; size: number; data: ArrayBuffer },
		meta: QueueMediaMeta
	) {
		await enqueuePost(
			circle.origin,
			circle.circleId,
			{ body: '', entry_date: todayEntryDate(), media_meta: [meta] },
			[file]
		);
		await loadFeedData();
	}

	async function sendVoice(take: VoiceTake) {
		if (preparing) return;
		preparing = 'Готовим голосовое…';
		error = '';
		try {
			const compression = await fetchCompression(circle.origin).catch(() => undefined);
			// Голосу хватает 64 кбит/с; перекодируем всегда — WebM с телефона
			// может не сыграть на iPhone.
			const { fallbackReason: _reason, ...audio } = await compressAudio(take.file, compression, undefined, {
				bitrateKbps: 64,
				force: true
			});
			await enqueueRecorded(
				{ ...audio, type: audioMime(audio.name, audio.type) },
				{
					kind: 'attachment',
					voice: true,
					audio_duration_ms: Math.round(take.durationMs),
					audio_peaks: take.peaks
				}
			);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			preparing = '';
		}
	}

	async function sendVideo(file: File) {
		if (preparing) return;
		preparing = 'Готовим видео…';
		error = '';
		try {
			const compression = await fetchCompression(circle.origin).catch(() => undefined);
			const { fallbackReason: _reason, ...video } = await compressVideo(
				file,
				compression,
				(progress) => {
					preparing = `Готовим видео… ${Math.round(progress * 100)}%`;
				},
				{ force: true }
			);
			const maxBytes = compression?.attachment_max_bytes ?? 0;
			if (maxBytes > 0 && video.size > maxBytes) {
				error = `Видео больше ${formatBytes(maxBytes)} — сервер не примет`;
				return;
			}
			await enqueueRecorded(video, { kind: 'video', is_cover: true });
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			preparing = '';
		}
	}

	function openComposeWithPhotos(files: File[]) {
		handComposePhotos(circle.circleId, files);
		openComposeFromBar();
	}

	function dismissDayPrompt() {
		dayPromptDate = '';
		void bumpDayPromptCount(circle.origin, circle.circleId);
	}

	function openDayFromPrompt() {
		const date = dayPromptDate;
		dayPromptDate = '';
		goto(`/circles/${circle.circleId}/days/${date}`);
	}

	function openCompose() {
		goto(`/circles/${circle.circleId}/compose`);
	}

	function openPost(postId: string) {
		leaveToPost(`/circles/${circle.circleId}/posts/${postId}`);
	}

	// Разовый параметр стирается из адреса через $app/navigation: прямой
	// history.replaceState проходил мимо роутера, и его состояние расходилось
	// с адресом (GUI-7).
	function dropSearchParam(name: string) {
		const url = new URL($page.url);
		if (!url.searchParams.has(name)) return Promise.resolve();
		url.searchParams.delete(name);
		return goto(`${url.pathname}${url.search}${url.hash}`, {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	function openAlbum(postId: string) {
		const post = posts.find((p) => p.id === postId);
		leaveToPost(albumHref(circle.circleId, postId, photoMedia(post?.media).length));
	}

	// Лист реакций — состояние экрана, а не шаг назад: открытие и закрытие
	// заменяют запись в истории. Без этого «назад» снова открывал лист, и до
	// улочки приходилось нажимать трижды (GUI-7).
	function openReactions(postId: string) {
		goto(`/circles/${circle.circleId}?reactions=${postId}`, {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	function closeReactions() {
		goto(`/circles/${circle.circleId}`, {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	function togglePicker(postId: string) {
		pickerPostId = pickerPostId === postId ? '' : postId;
	}

	const archiveHint =
		'Эта запись уйдёт с сервера в срок архивации. Новые комментарии и оценки к ней не принимаются.';

	function postArchiveLocked(post: FeedPost): boolean {
		return isPostArchiveLocked(
			Boolean(circle.archiveCycle?.active),
			circle.archiveCycle?.cutoff_date,
			post.created_at
		);
	}

	function showReactionPlus(post: FeedPost): boolean {
		return canReact(post.reactions, reactionActor, {
			canWrite: circle.canWrite,
			solo: soloCircle,
			locked: postArchiveLocked(post)
		});
	}

	function applyReaction(postId: string, emoji: string | null) {
		posts = posts.map((p) =>
			p.id === postId
				? { ...p, reactions: applyOwnReaction(p.reactions, postId, emoji, reactionActor) }
				: p
		);
	}

	async function pickReaction(post: FeedPost, emoji: string) {
		// Два тапа по одной реакции = поставить и снять (GUI-8).
		if (postArchiveLocked(post) || reacting) return;
		reacting = true;
		const mine = ownReaction(post.reactions, circle.identityId);
		const removing = mine?.emoji === emoji;
		error = '';
		try {
			if (navigator.onLine) {
				if (removing) {
					await removeReaction(circle.origin, circle.circleId, post.id);
				} else {
					await setReaction(circle.origin, circle.circleId, post.id, emoji);
				}
				pickerPostId = '';
				await loadFeedData();
			} else {
				if (removing) {
					await enqueueReactionRemove(circle.origin, circle.circleId, post.id);
					applyReaction(post.id, null);
				} else {
					await enqueueReaction(circle.origin, circle.circleId, {
						post_id: post.id,
						emoji
					});
					applyReaction(post.id, emoji);
				}
				pickerPostId = '';
			}
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			reacting = false;
		}
	}


	function formatIsoDay(iso: string, withYear = false): string {
		const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'long' };
		if (withYear) opts.year = 'numeric';
		return new Intl.DateTimeFormat('ru-RU', opts).format(new Date(iso));
	}

	function openInvite() {
		goto(`/circles/${circle.circleId}/settings/invite`);
	}

	function downloadArchiveClick() {
		if (circle.archiveCycle?.download_url) {
			void downloadArchive(circle.origin, circle.archiveCycle.download_url);
		}
	}

	function openQueued(id: number) {
		goto(`/circles/${circle.circleId}/compose?queue=${id}`);
	}

	function todayEntryDate(): string {
		const d = new Date();
		return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	}

	function isBackdated(post: FeedPost): boolean {
		return isAttachedToOtherDay(post);
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
	commentBar={circle.canWrite}
	onback={goBack}
	onsearch={openSearch}
	bind:commentDraft
	commentMembers={commentMembers}
	commentBusy={sending}
	onCommentSend={circle.canWrite ? sendFromBar : undefined}
	onCommentCompose={circle.canWrite ? openComposeFromBar : undefined}
	onCommentPhotos={circle.canWrite ? openComposeWithPhotos : undefined}
	onCommentVoice={circle.canWrite ? (take) => void sendVoice(take) : undefined}
	onCommentVideo={circle.canWrite ? () => (videoOpen = true) : undefined}
	commentStatus={preparing}
>
	{#if videoOpen}
		<VideoRecorder onsend={(file) => void sendVideo(file)} onclose={() => (videoOpen = false)} />
	{/if}
	<PullRefreshBand pull={ptr.state} />

	<div
		class="feed"
		role="feed"
		aria-label="Лента"
		bind:this={feedEl}
		onscroll={onFeedScroll}
		ontouchstart={ptr.start}
		ontouchmove={ptr.move}
		ontouchend={ptr.end}
	>
		{#if circle.archiveCycle?.active}
			<ArchiveBanner
				title="Архивация · {formatDeadline(circle.archiveCycle.deadline)}"
				onaction={downloadArchiveClick}
			>
				Всё до <strong>{formatEntryDate(circle.archiveCycle.cutoff_date)}</strong> будет удалено.
				Ваш архив — {formatBytes(circle.archiveCycle.personal_archive_bytes)}.
			</ArchiveBanner>
		{/if}

		{#if joinAvatarHint}
			<Hint class="gutter">{joinAvatarHint}</Hint>
		{/if}
		{#if !circle.canWrite}
			<Hint class="gutter">Вы читаете этот круг и не пишете.</Hint>
		{/if}
		<!-- «?loading» — посмотреть экран загрузки в круге, как «/?loading». -->
		{#if loading || $page.url.searchParams.has('loading')}
			<Loading />
		{:else if error && !posts.length}
			<Hint class="gutter-24">{error}</Hint>
		{:else}
			{#each circle.canWrite ? queuedPosts : [] as item (item.id)}
				<!-- Snippets live outside <PostCard>: a {#snippet} nested in {#if} is not passed as a prop. -->
				{#snippet queuedMedia()}
					<MediaTile variant="feed" count={item.file_count} />
				{/snippet}
				{#snippet queuedError()}
					<Hint>{item.error}</Hint>
				{/snippet}
				<PostCard
					queued={true}
					onclick={() => openQueued(item.id)}
					media={item.file_count ? queuedMedia : undefined}
					comments={item.state === 'failed' && item.error ? queuedError : undefined}
				>
						{#snippet author()}
							<PostByline
								initial={circle.identityInitial}
								color={circle.colorHex}
								src={circle.avatarUrl}
								name={circle.identityName}
								time="в очереди"
								icon="clock"
							/>
						{/snippet}
						{#snippet headerRight()}
							<IconButton
								name="trash"
								size="sm"
								label="Удалить из очереди"
								stopPropagation
								onclick={() => (queueToRemove = item.id)}
							/>
						{/snippet}
						{#snippet text()}
							{item.body || 'Без текста'}
						{/snippet}
					</PostCard>
			{/each}

			{#each serviceEventsAboveNewest(feedEvents, posts[0]?.event_seq) as ev (ev.seq)}
				<EventDivider text={ev.summary} />
			{/each}

			{#each posts as post, i (post.id)}
				{#if dividerAt === i}
					<EventDivider variant="unread" text="выше — новое" />
				{/if}
				<!-- Snippets live outside <PostCard>: a {#snippet} nested in {#if} is not passed as a prop. -->
				{#snippet postDate()}
					<EntryDateMark label={formatEntryDate(post.entry_date)} />
				{/snippet}
				{#snippet postText()}
					<MentionText body={post.body} />
				{/snippet}
				{#snippet postMedia()}
					{@const cover = coverMedia(post.media)}
					{@const count = mediaCount(post.media)}
					{@const loc = locationLabel(cover)}
					{#if cover}
						<MediaTile
							variant="feed"
							src={mediaUrls[cover.blob_id]}
							kind={cover.kind === 'video' ? 'video' : 'photo'}
							crop={cover.crop}
							loading={mediaProgress[cover.blob_id]}
							failed={mediaFailed[cover.blob_id]}
							{count}
							locationLabel={loc || undefined}
							onclick={() => openAlbum(post.id)}
						/>
					{/if}
					<AttachmentList
						items={attachmentMedia(post.media)}
						origin={circle.origin}
						circleId={circle.circleId}
						circleName={circle.name}
						color={circle.colorHex}
						postId={post.id}
						coverUrls={mediaUrls}
					/>
				{/snippet}
				<PostCard
					onclick={() => openPost(post.id)}
					headerRight={isBackdated(post) ? postDate : undefined}
					text={post.body ? postText : undefined}
					media={coverMedia(post.media) || attachmentMedia(post.media).length
						? postMedia
						: undefined}
				>
					{#snippet author()}
						<PostByline
							initial={authorInitial(post.author_name)}
							color={circle.colorHex}
							src={authorAvatarSrc(post)}
							name={post.author_name}
							time={formatPostTime(post.created_at)}
						/>
					{/snippet}
					{#snippet reactions()}
						{#if !soloCircle}
							<ReactionBar
								groups={groupReactions(post.reactions).map((g) => ({
									icon: reactionIconName(g.emoji),
									names: g.names
								}))}
								keys={REACTION_KEYS}
								showAdd={showReactionPlus(post) && pickerPostId !== post.id}
								pickerOpen={pickerPostId === post.id}
								selectedKey={ownReaction(post.reactions, circle.identityId)?.emoji ?? ''}
								onopenList={() => openReactions(post.id)}
								onadd={() => togglePicker(post.id)}
								onpick={(key) => void pickReaction(post, key)}
							/>
						{/if}
					{/snippet}
				</PostCard>
				{#if postArchiveLocked(post)}
					<Hint class="m-0-16-12">{archiveHint}</Hint>
				{/if}
				{#if post.comments?.length}
					{@const preview = commentPreview(post.comments)}
					<CommentPreview
						first={preview.first || undefined}
						time={preview.createdAt ? formatPostTime(preview.createdAt) : undefined}
						more={preview.more ? `ещё ${plural(preview.more, WORD.comment)}` : undefined}
						onclick={() => openPost(post.id)}
					/>
				{/if}
				{#if dayPromptDate && circle.canWrite && post.entry_date === dayPromptDate && i === posts.findIndex((p) => p.entry_date === dayPromptDate)}
					<FeedDayPromptCard
						title="Первая запись за {formatEntryDate(dayPromptDate)}"
						primaryLabel="Назвать день"
						secondaryLabel="Потом"
						onprimary={openDayFromPrompt}
						onsecondary={dismissDayPrompt}
					>
						Дать этому дню название и обложку? День общий: увидят все, и поправить сможет любой, у
						кого есть за него запись.
					</FeedDayPromptCard>
				{/if}
				{#each serviceEventsBetween(
					feedEvents,
					post.event_seq,
					i + 1 < posts.length ? posts[i + 1].event_seq : 0
				) as ev (ev.seq)}
					<EventDivider text={ev.summary} />
				{/each}
			{/each}

			{#if dividerAt === posts.length}
				<EventDivider variant="unread" text="выше — новое" />
			{/if}

			{#if loadingOlder}
				<Hint class="gutter" centered>Загружаем записи постарше…</Hint>
			{/if}

			{#if error}
				<Hint class="gutter">{error}</Hint>
			{/if}

			{#if !posts.length && !queuedPosts.length}
				<!-- Читатель не пишет и не зовёт: призыв и «Пригласить» — только пишущим. -->
				{#snippet callToWrite()}
					Напишите первым — или позовите тех, с кем хотите это вести.
				{/snippet}
				{#snippet inviteAction()}
					<Button variant="colored" onclick={openInvite}>Пригласить</Button>
				{/snippet}
				<EmptyState
					title="Пока ничего"
					place="feed"
					children={circle.canWrite ? callToWrite : undefined}
					actions={circle.canWrite ? inviteAction : undefined}
				/>
			{:else if posts.length}
				<FeedEnd
					since={showVisibilityCutoff && visibleFrom ? formatIsoDay(visibleFrom) : undefined}
					started={circleStartedAt
						? formatIsoDay(circleStartedAt, Boolean(showVisibilityCutoff && visibleFrom))
						: undefined}
				/>
			{/if}
		{/if}
	</div>

	{#if reactionsPost}
		<ReactionsSheet reactions={reactionsPost.reactions ?? []} color={circle.colorHex} ondismiss={closeReactions} />
	{/if}

	{#if queueToRemove !== null}
		<ConfirmDialog
			title="Удалить неотправленную запись?"
			confirmLabel="Удалить"
			cancelLabel="Оставить"
			onconfirm={() => void confirmQueueRemove()}
			oncancel={() => (queueToRemove = null)}
		>
			<Hint>Она ещё не ушла на сервер. Удалить — значит потерять текст и снимки.</Hint>
		</ConfirmDialog>
	{/if}
</CircleLayout>
