<script lang="ts">
	import { serverHost } from '$lib/circles/move';
	import { spaceNotice } from '$lib/circles/space';
	import { navigateBack } from '$lib/navigation/up';
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
	import FeedDayCard from '$ui/data/FeedDayCard.svelte';
	import FeedDayPromptCard from '$ui/data/FeedDayPromptCard.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import AttachmentRow from '$ui/data/AttachmentRow.svelte';
	import CommentMedia from '$ui/data/CommentMedia.svelte';
	import CommentPreview from '$ui/data/CommentPreview.svelte';
	import VoiceRow from '$ui/data/VoiceRow.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import { downloadBlob } from '$lib/media/objectUrl';
	import { WORD, plural } from '$lib/format/plural';
	import {
		formatCutoffDate,
		formatDeadline,
		formatEntryDate,
		formatPostTime,
		formatSessionDay
	} from '$lib/format/time';
	import { isPostArchiveLocked } from '$lib/journal/archive';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { circleReading } from '$lib/journal/reading.svelte';
	import { canReact } from '$lib/journal/reactions';
	import { fetchMembers, type MemberInfo } from '$lib/circles/settings';
	import {
		isAttachedToOtherDay,
		attachmentMedia,
		authorInitial,
		commentMediaItems,
		commentPreview,
		coverMedia,
		type CommentMediaItem,
		dayCoverTileId,
		groupReactions,
		locationLabel,
		mediaCount,
		maxReadSeq,
		ownReaction,
		reactionIconName,
		REACTION_KEYS,
		serviceEventsBetween,
		serviceEventsAboveNewest,
		albumHref,
		photoMedia,
		tileBlobId
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
	import { videoDurationMs, videoPosterJpeg } from '$lib/media/videoPoster';
	import type { VoiceTake } from '$lib/media/voiceRecorder.svelte';
	import type { QueueMediaMeta } from '$lib/idb/db';
	import VideoRecorder from '$ui/overlays/VideoRecorder.svelte';
	import { loadDays } from '$lib/journal/days';
	import type { FeedEvent, FeedPost } from '$lib/journal/types';
	import { bumpDayPromptCount, getDayPromptShowCount, isDayPromptDismissed } from '$lib/idb/db';
	import {
		enqueuePost,
		enqueueReaction,
		enqueueReactionRemove,
		listQueuedPosts,
		queuedTimeLabel,
		removeQueueItem,
		subscribePostSent,
		subscribeQueue,
		subscribeQueueProgress,
		type QueuedPostView
	} from '$lib/queue/queue';
	import { isTransportError } from '$lib/queue/transport';
	import { registerRefetch } from '$lib/sync/sync';
	import { authErrorHint } from '$lib/auth/auth';
	import { handComposePhotos } from '$lib/journal/compose-handoff';
	import { holdKeyboard } from '$lib/navigation/keyboard';
	import { applyFeedSpot, feedSpotKey, saveFeedSpot, takeFeedSpot } from '$lib/journal/feed-scroll';
	import { mark, markPainted } from '$lib/dev/marks';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	// Места нет (C42): читать и удалять своё можно, писать — нет (3.10, 3.11).
	const canSay = $derived(circle.canWrite && !circle.spaceFull);
	const notice = $derived(spaceNotice(circle));

	// Флаги «идёт отправка»: защита от второго нажатия (GUI-8).
	let sending = $state(false);
	let reacting = $state(false);
	// Подтверждение удаления неотправленной записи (STB-6).
	let queueToRemove = $state<number | null>(null);

	let members = $state<MemberInfo[]>([]);
	let activeMemberCount = $state(2);
	// Себя не упоминают: в списке — остальные участники круга.
	const commentMembers = $derived(
		members.filter((m) => m.status === 'active' && m.identity_id !== circle.identityId)
	);
	let queuedPosts = $state<QueuedPostView[]>([]);
	let joinAvatarHint = $state('');
	let fixedLastRead = $state(0);
	// Данные ленты живут в контроллере $lib: первая порция, догрузка старших,
	// граница непрочитанного и ошибка — с поведенческими тестами (план 60).
	// Контроллер общий с экраном записи и держится, пока открыт круг (C95-1):
	// возврат из записи не перечитывает ленту, обложки и лица — те же.
	const reading = circleReading(circle.origin, circle.circleId, () => fixedLastRead);
	const feed = reading.feed;
	// Разметка читает поля контроллера под прежними именами.
	let posts = $derived(feed.posts);
	let loadingOlder = $derived(feed.loadingOlder);
	let feedEvents = $derived(feed.feedEvents);
	let visibleFrom = $derived(feed.visibleFrom);
	let circleStartedAt = $derived(feed.circleStartedAt);
	let loading = $derived(feed.loading);
	let error = $derived(feed.error);
	let dividerAt = $derived(feed.dividerAt);
	let feedEl: HTMLDivElement | undefined = $state();
	// Автомат жеста живёт в $lib и покрыт тестами (GUI-5).
	// Обновление жестом (3.5): тянешь ленту от верха — она перечитывается.
	const ptr = new PullRefresh(
		() => feedEl?.scrollTop ?? 0,
		() => loadFeedData()
	);
	let commentDraft = $state('');
	let dayPromptDate = $state('');
	let pickerPostId = $state('');

	const reactionsPostId = $derived($page.url.searchParams.get('reactions'));
	const reactionsPost = $derived(
		reactionsPostId ? posts.find((p) => p.id === reactionsPostId) : undefined
	);

	function authorAvatarSrc(post: FeedPost): string | undefined {
		if (post.identity_id === circle.identityId && circle.avatarUrl) return circle.avatarUrl;
		return reading.authorAvatarUrls[post.identity_id];
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
		// День, открытый с открытки (3.20), — тот же уход вглубь, что и запись.
		const path = from?.url.pathname ?? '';
		cameFromPost =
			path.startsWith(`/circles/${circle.circleId}/posts/`) ||
			path.startsWith(`/circles/${circle.circleId}/days/`);
		void restoreSpot();
	});

	async function restoreSpot() {
		if (spotDone || cameFromPost === undefined || !feedLoaded) return;
		spotDone = true;
		const spot = takeFeedSpot(feedSpotKey(circle.origin, circle.circleId));
		if (!spot || !cameFromPost) return;
		mark('d:restore-start');
		await tick();
		if (feedEl)
			applyFeedSpot(
				feedEl,
				posts.map((p) => p.id),
				spot
			);
		void markPainted('r:feed-restored');
	}

	function leaveToPost(path: string) {
		saveFeedSpot(
			feedSpotKey(circle.origin, circle.circleId),
			feedEl,
			posts.map((p) => p.id)
		);
		goto(path);
	}

	async function loadFeedData() {
		mark('t:feed-load-start');
		await feed.load();
		feedLoaded = true;
		void markPainted('r:feed-items');
		await restoreSpot();
	}

	// До конца ленты — полтора экрана: догружаем заранее, чтобы не упираться.
	function onFeedScroll() {
		const el = feedEl;
		if (!el || !feed.nextBefore) return;
		if (el.scrollTop + el.clientHeight * 2.5 >= el.scrollHeight) void feed.loadOlder();
	}

	// Плитка записи в очереди показывает сам снимок (или кадр ролика): он ещё
	// на устройстве, скачивать нечего. Адреса живут, пока запись в очереди.
	let queuedPreviewUrls = $state<Record<number, string>>({});
	// Доля ушедших байт у записи, которая отправляется сейчас.
	let sendProgress = $state<Record<number, number>>({});

	function refreshQueued() {
		void listQueuedPosts(circle.origin, circle.circleId).then((items) => {
			const next: Record<number, string> = {};
			for (const item of items) {
				const preview = item.media.find((m) => m.kind !== 'attachment')?.preview;
				if (!preview) continue;
				next[item.id] =
					queuedPreviewUrls[item.id] ??
					URL.createObjectURL(new Blob([preview.data], { type: preview.type }));
			}
			for (const [id, url] of Object.entries(queuedPreviewUrls)) {
				if (!next[Number(id)]) URL.revokeObjectURL(url);
			}
			queuedPreviewUrls = next;
			queuedPosts = items;
		});
	}

	onMount(() => {
		if ($page.url.searchParams.get('joinAvatar') === 'fail') {
			joinAvatarHint = 'Фото не загрузилось — поставьте в профиле';
			void dropSearchParam('joinAvatar');
		}
		fixedLastRead = circle.lastReadSeq;
		// Лента могла загрузиться параллельно с проверкой доступа (C95-2) —
		// граница непрочитанного теперь считается по известному курсору.
		reading.redrawDivider();
		const dayPromptParam = $page.url.searchParams.get('dayPrompt');
		if (feed.posts.length || feed.error) {
			// Возврат из записи или лента, догрузившаяся параллельно с проверкой
			// доступа (C95-1, C95-2): карточки на экране сразу, перечитывать
			// не нужно. Свежесть приносит поток синхронизации и фоновые
			// обновления.
			feedLoaded = true;
			void markPainted('r:feed-items');
			void restoreSpot();
		} else {
			void loadFeedData().then(() => {
				if (!dayPromptParam || !/^\d{4}-\d{2}-\d{2}$/.test(dayPromptParam)) return;
				void dropSearchParam('dayPrompt');
				void checkDayPrompt(dayPromptParam);
			});
		}
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
			refetch: loadFeedData
		});
		// Ушедшая запись встаёт в ленту до того, как пропадёт из очереди.
		const unsubSent = subscribePostSent(async (origin, circleId) => {
			if (origin === circle.origin && circleId === circle.circleId) await loadFeedData();
		});
		return () => {
			unsubQueue();
			unsubProgress();
			unsubSync();
			unsubSent();
		};
	});

	onDestroy(() => {
		ptr.destroy();
		for (const url of Object.values(queuedPreviewUrls)) URL.revokeObjectURL(url);
		const seq = maxReadSeq(posts);
		if (seq > 0) {
			void advanceReadCursor(circle.origin, circle.circleId, seq);
		}
	});

	function goBack() {
		mark('t:tap-back-list');
		void navigateBack();
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
		feed.clearError();
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
			feed.setError(authErrorHint(err));
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

	// Голосовое и видео из полосы ввода (C14, 4.13, 4.15): новая запись журнала
	// без текста — «Запись» стоит только у пустого поля. Через очередь — без
	// сети тоже уйдёт.
	let videoOpen = $state(false);
	let videoSettings = $state<Awaited<ReturnType<typeof fetchCompression>> | undefined>();

	// Камере нужны настройки сжатия сервера: она пишет сразу в этот размер.
	async function openVideo() {
		videoSettings = await fetchCompression(circle.origin).catch(() => undefined);
		videoOpen = true;
	}
	let preparing = $state('');
	// Что вышло с записью — над полосой ввода, там же, где «Готовим видео…».
	// Строка ошибки стоит под лентой: её не видно, и её стирает первая же
	// перезагрузка ленты после отправки.
	let barNote = $state('');
	let barNoteTimer: ReturnType<typeof setTimeout> | undefined;

	function showBarNote(text: string) {
		clearTimeout(barNoteTimer);
		barNote = text;
		barNoteTimer = setTimeout(() => (barNote = ''), 12000);
	}

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
		feed.clearError();
		try {
			const compression = await fetchCompression(circle.origin).catch(() => undefined);
			// Голосу хватает 64 кбит/с; перекодируем всегда — WebM с телефона
			// может не сыграть на iPhone.
			const { fallbackReason: _reason, ...audio } = await compressAudio(
				take.file,
				compression,
				undefined,
				{
					bitrateKbps: 64,
					force: true
				}
			);
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
			showBarNote(authErrorHint(err));
		} finally {
			preparing = '';
		}
	}

	async function sendVideo(file: File, livePoster?: ArrayBuffer, recordedMs?: number) {
		if (preparing) return;
		preparing = 'Готовим видео…';
		feed.clearError();
		try {
			const compression = await fetchCompression(circle.origin).catch(() => undefined);
			const { fallbackReason, ...video } = await compressVideo(
				file,
				compression,
				(progress) => {
					preparing = `Готовим видео… ${Math.round(progress * 100)}%`;
				},
				{ force: true }
			);
			const maxBytes = compression?.attachment_max_bytes ?? 0;
			if (maxBytes > 0 && video.size > maxBytes) {
				showBarNote(`Видео больше ${formatBytes(maxBytes)} — сервер не примет`);
				return;
			}
			// Кадр с камеры надёжнее: с готового файла его отдаёт не каждый браузер.
			const blob = new Blob([video.data], { type: video.type || 'video/mp4' });
			const poster = livePoster ?? (await videoPosterJpeg(blob));
			// Длительность для угла плитки (4.16а): из файла, иначе — сколько шла запись.
			const durationMs = (await videoDurationMs(blob)) ?? recordedMs;
			await enqueueRecorded(video, {
				kind: 'video',
				is_cover: true,
				video_poster: poster ? { type: 'image/jpeg', data: poster } : undefined,
				video_duration_ms: durationMs
			});
			// Ушло как записано — сказать, а не молчать. Где кодировщика нет
			// вовсе, об этом уже сказал экран перед отправкой (4.15).
			if (fallbackReason && typeof VideoEncoder !== 'undefined') {
				showBarNote(`Видео ушло без сжатия: ${fallbackReason}`);
			}
		} catch (err) {
			showBarNote(authErrorHint(err));
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

	// ?from=feed — «Назад» со дня вернёт в ленту, на то же место, а не во
	// вкладку «Дни» (план 46, C29). В адресе, а не в памяти экрана: день
	// строится заново после каждого захода в запись или альбом.
	function openDay(entryDate: string) {
		leaveToPost(`/circles/${circle.circleId}/days/${entryDate}?from=feed`);
	}

	function openPost(postId: string) {
		mark('t:tap-post');
		leaveToPost(`/circles/${circle.circleId}/posts/${postId}`);
	}

	// Снимок из превью комментария (4.22) открывается во весь экран в обсуждении.
	function openCommentPhoto(postId: string, commentId: string, index: number) {
		leaveToPost(
			`/circles/${circle.circleId}/posts/${postId}?cphoto=${encodeURIComponent(commentId)}&lb=${index}`
		);
	}

	function downloadCommentFile(item: CommentMediaItem) {
		if (item.blobId) void downloadBlob(circle.origin, item.blobId, item.name || 'файл');
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

	// Граница архивации (владелец, 2026-10-15): одна строка над первой записью
	// старше отсечки, вид — как «выше — новое»; что реакции и комментарии
	// закрыты, говорит полоса «Архивация» наверху.
	const archiveLineText = $derived(
		`ниже — удалится ${formatSessionDay(circle.archiveCycle?.deadline ?? '')}`
	);
	const archiveLineAt = $derived.by(() => {
		const i = posts.findIndex((p) => postArchiveLocked(p));
		return i < 0 ? null : i;
	});

	function postArchiveLocked(post: FeedPost): boolean {
		return isPostArchiveLocked(
			Boolean(circle.archiveCycle?.active),
			circle.archiveCycle?.cutoff_date,
			post.created_at
		);
	}

	function showReactionPlus(post: FeedPost): boolean {
		return canReact(post.reactions, reactionActor, {
			canWrite: canSay,
			solo: soloCircle,
			locked: postArchiveLocked(post)
		});
	}

	async function pickReaction(post: FeedPost, emoji: string) {
		// Два тапа по одной реакции = поставить и снять (GUI-8).
		if (postArchiveLocked(post) || reacting) return;
		reacting = true;
		const mine = ownReaction(post.reactions, circle.identityId);
		const removing = mine?.emoji === emoji;
		feed.clearError();
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
					feed.applyReaction(post.id, null, reactionActor);
				} else {
					await enqueueReaction(circle.origin, circle.circleId, {
						post_id: post.id,
						emoji
					});
					feed.applyReaction(post.id, emoji, reactionActor);
				}
				pickerPostId = '';
			}
		} catch (err) {
			feed.setError(authErrorHint(err));
		} finally {
			reacting = false;
		}
	}

	function formatIsoDay(iso: string, withYear = false): string {
		const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'long' };
		if (withYear) opts.year = 'numeric';
		// Без «г.» после года: «круг живёт с 20 октября 2026».
		return new Intl.DateTimeFormat('ru-RU', opts).format(new Date(iso)).replace(/\s*г\.$/, '');
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

<!-- Полоса вместо строки ввода, когда места нет (3.10, 3.11) и когда круг
     замер (3.16). -->
{#snippet spaceNote()}
	{#if circle.frozenAt && circle.move?.status === 'accepted'}
		Круг переехал — писать можно на новом сервере
	{:else if circle.frozenAt && circle.move}
		Круг замер на время переезда
	{:else if circle.frozenAt}
		Круг замер: его убирают с сервера
	{:else if notice}
		{notice.first}<br />
		{#if notice.href}
			<a class="under" href={notice.href}>{notice.second}</a>
		{:else}
			{notice.second}
		{/if}
	{/if}
{/snippet}

<CircleLayout
	app
	color={circle.color}
	title={circle.name}
	identity={circle.identityName}
	avatar={circle.identityInitial}
	avatarSrc={circle.avatarUrl}
	circleId={circle.circleId}
	identitySettingsLink={circle.canWrite}
	commentBar={canSay}
	notice={notice || circle.frozenAt ? spaceNote : undefined}
	onback={goBack}
	onsearch={openSearch}
	bind:commentDraft
	{commentMembers}
	commentBusy={sending}
	onCommentSend={canSay ? sendFromBar : undefined}
	onCommentCompose={canSay ? openComposeFromBar : undefined}
	onCommentPhotos={canSay ? openComposeWithPhotos : undefined}
	commentPreservePhotoLocation
	onCommentPhotoError={() => feed.setError('Не удалось открыть выбранные фото. Попробуйте ещё раз')}
	onCommentVoice={canSay ? (take) => void sendVoice(take) : undefined}
	onCommentVideo={canSay ? () => void openVideo() : undefined}
	commentStatus={preparing || barNote}
>
	{#if videoOpen}
		<VideoRecorder
			settings={videoSettings}
			onsend={(file, poster, durationMs) => void sendVideo(file, poster, durationMs)}
			onclose={() => (videoOpen = false)}
		/>
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
		{#if circle.frozenAt && circle.move?.status === 'accepted'}
			<!-- 3.15: новый сервер принял круг — куда идти и какой почтой входить. -->
			{@const dest = serverHost(circle.move.dest_url)}
			<ArchiveBanner
				title="Круг переехал"
				action={circle.youOwn ? 'Завершить перенос круга' : 'Перейти на новый сервер'}
				onaction={circle.youOwn
					? () => goto(`/circles/${circle.circleId}/move`)
					: () => {
							// C69: новый сервер — его собственная страница, там и вход;
							// добавить его сюда веб-приложение пока не может.
							if (circle.move?.dest_url) window.location.assign(circle.move.dest_url);
						}}
			>
				«{circle.name}» теперь на <strong>{dest}</strong>. Войдите там почтой {circle.move.email} — попадёте
				в своё имя и свои записи. Здесь круг замер и исчезнет после завершения переноса.
			</ArchiveBanner>
		{:else if circle.frozenAt && circle.move}
			<!-- 3.14: круг переезжает; куда — старый сервер пока не знает. -->
			<ArchiveBanner
				title="Переезд · круг замер"
				action={circle.youOwn ? 'Перенос круга' : undefined}
				onaction={circle.youOwn ? () => goto(`/circles/${circle.circleId}/move`) : undefined}
			>
				«{circle.name}» переезжает на другой сервер. Читать можно, писать нельзя.
			</ArchiveBanner>
		{:else if circle.frozenAt}
			<!-- Общий текст для обеих ролей; различаются доступные действия (client-reference). -->
			<ArchiveBanner
				title="Круг уходит с сервера"
				action={circle.youOwn ? 'Полный архив круга' : 'Скачать свой архив'}
				onaction={circle.youOwn
					? () => goto(`/circles/${circle.circleId}/quota/full`)
					: () =>
							void downloadArchive(
								circle.origin,
								`/api/v1/circles/${circle.circleId}/archive/download`
							)}
			>
				Круг «{circle.name}» сохраняют в файл и убирают с этого сервера. Читать можно, писать
				нельзя. Свой архив — всё, что вы видели в круге, — можно скачать, пока круг здесь.
			</ArchiveBanner>
		{:else if circle.archiveCycle?.active}
			<ArchiveBanner
				title="Архивация · {formatDeadline(circle.archiveCycle.deadline)}"
				onaction={circle.archiveCycle.download_url ? downloadArchiveClick : undefined}
			>
				Всё до <strong>{formatCutoffDate(circle.archiveCycle.cutoff_date)}</strong> будет удалено.
				Реакции и комментарии к этим записям закрыты.
				<!-- Размер — только файлы; без них «0 Б» читалось как пустой архив. -->
				{#if circle.archiveCycle.personal_archive_bytes}
					Ваш архив — {formatBytes(circle.archiveCycle.personal_archive_bytes)}.
				{/if}
			</ArchiveBanner>
		{/if}

		{#if joinAvatarHint}
			<Hint class="gutter">{joinAvatarHint}</Hint>
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
					{@const visual = item.media.filter((m) => m.kind !== 'attachment')}
					{#if visual.length}
						<MediaTile
							variant="feed"
							src={queuedPreviewUrls[item.id]}
							kind={visual[0].kind === 'video' ? 'video' : 'photo'}
							durationMs={visual[0].duration_ms}
							count={visual.length}
						/>
					{/if}
					{#each item.media.filter((m) => m.kind === 'attachment') as att, ai (ai)}
						{#if att.voice}
							<VoiceRow blobId="" peaks={att.peaks ?? []} durationMs={att.duration_ms ?? 0} />
						{:else}
							<AttachmentRow filename={att.name} size={formatBytes(att.size)} />
						{/if}
					{/each}
				{/snippet}
				{#snippet queuedError()}
					<Hint>{item.error}</Hint>
				{/snippet}
				{#snippet queuedText()}
					{item.body}
				{/snippet}
				<PostCard
					queued={true}
					onclick={() => openQueued(item.id)}
					text={item.body ? queuedText : undefined}
					media={item.file_count ? queuedMedia : undefined}
					comments={item.state === 'failed' && item.error ? queuedError : undefined}
				>
					{#snippet author()}
						<PostByline
							initial={circle.identityInitial}
							color={circle.colorHex}
							src={circle.avatarUrl}
							name={circle.identityName}
							time={queuedTimeLabel(item.state, sendProgress[item.id], item.error)}
							icon="clock"
						/>
					{/snippet}
					{#snippet headerRight()}
						<!-- C44: явная правка рядом с удалением; тело карточки тоже открывает редактор. -->
						<IconButton
							class="ml-auto"
							name="edit"
							size="sm"
							label="Править запись в очереди"
							stopPropagation
							onclick={() => openQueued(item.id)}
						/>
						<IconButton
							name="trash"
							size="sm"
							label="Удалить из очереди"
							stopPropagation
							onclick={() => (queueToRemove = item.id)}
						/>
					{/snippet}
				</PostCard>
			{/each}

			<!-- Название и обложка дня — открытка (3.20), остальное — тонкая строка. -->
			{#snippet feedEvent(ev: FeedEvent)}
				{#if ev.day}
					{@const coverId = dayCoverTileId(ev.day)}
					<FeedDayCard
						date={formatEntryDate(ev.day.entry_date)}
						title={ev.day.title}
						hasCover={Boolean(coverId)}
						coverUrl={coverId ? reading.mediaUrls[coverId] : undefined}
						video={ev.day.cover_is_video}
						caption="{ev.day.caption} · {formatPostTime(ev.created_at)}"
						onclick={() => openDay(ev.day!.entry_date)}
					/>
				{:else}
					<EventDivider text={ev.summary} time={formatPostTime(ev.created_at)} />
				{/if}
			{/snippet}

			{#each serviceEventsAboveNewest(feedEvents, posts[0]?.event_seq) as ev (ev.seq)}
				{@render feedEvent(ev)}
			{/each}

			{#each posts as post, i (post.id)}
				{#if dividerAt === i}
					<EventDivider variant="unread" text="выше — новое" />
				{/if}
				{#if archiveLineAt === i}
					<EventDivider variant="cutoff" text={archiveLineText} />
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
							src={reading.mediaUrls[tileBlobId(cover)]}
							kind={cover.kind === 'video' ? 'video' : 'photo'}
							durationMs={cover.video_duration_ms}
							crop={cover.crop}
							loading={reading.mediaProgress[tileBlobId(cover)]}
							failed={reading.mediaFailed[tileBlobId(cover)]}
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
						coverUrls={reading.mediaUrls}
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
				{#if post.comments?.length}
					{@const preview = commentPreview(post.comments)}
					{@const last = preview.comment}
					{#snippet previewMedia()}
						<CommentMedia
							items={commentMediaItems(last?.media, reading.mediaUrls)}
							origin={circle.origin}
							audio={{
								circleId: circle.circleId,
								circleName: circle.name,
								color: circle.colorHex,
								postId: post.id
							}}
							onphoto={(n) => last && openCommentPhoto(post.id, last.id, n)}
							onfile={downloadCommentFile}
						/>
					{/snippet}
					<CommentPreview
						first={preview.first || undefined}
						time={preview.createdAt ? formatPostTime(preview.createdAt) : undefined}
						more={preview.more ? `ещё ${plural(preview.more, WORD.comment)}` : undefined}
						media={last?.media?.length ? previewMedia : undefined}
						onclick={() => openPost(post.id)}
					/>
				{/if}
				{#if dayPromptDate && canSay && post.entry_date === dayPromptDate && i === posts.findIndex((p) => p.entry_date === dayPromptDate)}
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
				{#each serviceEventsBetween(feedEvents, post.event_seq, i + 1 < posts.length ? posts[i + 1].event_seq : 0) as ev (ev.seq)}
					{@render feedEvent(ev)}
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
					children={canSay ? callToWrite : undefined}
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
		<ReactionsSheet
			reactions={reactionsPost.reactions ?? []}
			color={circle.colorHex}
			ondismiss={closeReactions}
		/>
	{/if}

	{#if queueToRemove !== null}
		<ConfirmDialog
			title="Удалить неотправленную запись?"
			confirmLabel="Удалить"
			cancelLabel="Оставить"
			onconfirm={() => void confirmQueueRemove()}
			oncancel={() => (queueToRemove = null)}
		/>
	{/if}
</CircleLayout>
