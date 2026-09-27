<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onDestroy, onMount } from 'svelte';
	import ArchiveBanner from '$ui/data/ArchiveBanner.svelte';
	import AttachmentRow from '$ui/data/AttachmentRow.svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import EntryDateMark from '$ui/data/EntryDateMark.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import EventDivider from '$ui/data/EventDivider.svelte';
	import FeedDayPromptCard from '$ui/data/FeedDayPromptCard.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import Icon from '$ui/Icon.svelte';
	import Mark from '$ui/Mark.svelte';
	import CommentPreview from '$ui/data/CommentPreview.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import ReactionListRow from '$ui/data/ReactionListRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
	import { isAccessError } from '$lib/api/client';
	import {
		PTR,
		pullEnd,
		pullFinished,
		pullHeight,
		pullIdle,
		pullMarkHeight,
		pullMove,
		pullSettled,
		pullStart,
		pullVisible
	} from '$lib/gestures/pullToRefresh';
	import { formatBytes } from '$lib/format/bytes';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { WORD, plural } from '$lib/format/plural';
	import { formatDeadline, formatEntryDate, formatPostTime, isEditableActive } from '$lib/format/time';
	import { isPostArchiveLocked } from '$lib/journal/archive';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import { splitMentionBody } from '$lib/journal/mentions';
	import { applyOwnReaction, canReact } from '$lib/journal/reactions';
	import { fetchMembers, type MemberInfo } from '$lib/circles/settings';
	import {
		attachmentLabel,
		attachmentMedia,
		attachmentSizeLabel,
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
		unreadDividerIndex
	} from '$lib/journal/present';
	import { advanceReadCursor } from '$lib/journal/read-cursor';
	import { createPost, downloadArchive, removeReaction, setReaction } from '$lib/journal/posts';
	import { loadDays } from '$lib/journal/days';
	import type { FeedEvent, FeedPost } from '$lib/journal/types';
	import {
		bumpDayPromptCount,
		getDayPromptShowCount,
		isDayPromptDismissed
	} from '$lib/idb/db';
	import { downloadBlob } from '$lib/media/objectUrl';
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

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	// Флаги «идёт отправка»: защита от второго нажатия (GUI-8).
	let sending = $state(false);
	let reacting = $state(false);
	// Подтверждение удаления неотправленной записи (STB-6).
	let queueToRemove = $state<number | null>(null);

	let posts = $state<FeedPost[]>([]);
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
	let pull = $state(pullIdle());
	let ptrTimer: ReturnType<typeof setTimeout> | null = null;
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
		await resolveMediaUrls(circle.origin, [...coverJobs.keys()], (blobId, url) => {
			mediaUrls = { ...mediaUrls, [blobId]: url };
		});
	}

	const soloCircle = $derived(activeMemberCount === 1);
	const reactionActor = $derived({
		identityId: circle.identityId,
		identityName: circle.identityName
	});
	const showVisibilityCutoff = $derived(Boolean(visibleFrom));
	const ptrHeight = $derived(pullHeight(pull));

	async function loadFeedData() {
		error = '';
		try {
			const snap = await loadFeed(circle.origin, circle.circleId);
			posts = snap.posts;
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
			pull = pullFinished();
		}
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
		if (ptrTimer !== null) clearTimeout(ptrTimer);
		const seq = maxReadSeq(posts);
		if (seq > 0) {
			void advanceReadCursor(circle.origin, circle.circleId, seq);
		}
	});

	function goBack() {
		goto('/circles');
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
		goto(`/circles/${circle.circleId}/compose`);
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
		goto(`/circles/${circle.circleId}/posts/${postId}`);
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
		goto(`/circles/${circle.circleId}/posts/${postId}/album`);
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

	function onTouchStart(e: TouchEvent) {
		if (!feedEl) return;
		pull = pullStart(pull, e.touches[0]?.clientY ?? 0, feedEl.scrollTop);
	}

	function onTouchMove(e: TouchEvent) {
		if (!feedEl) return;
		pull = pullMove(pull, e.touches[0]?.clientY ?? 0, feedEl.scrollTop);
	}

	function onTouchEnd() {
		pull = pullEnd(pull);
		if (pull.phase !== 'settling') return;
		// Таймер снимается при уходе с экрана: раньше он доживал до
		// размонтированного компонента (GUI-5).
		ptrTimer = setTimeout(() => {
			ptrTimer = null;
			pull = pullSettled(pull);
			void loadFeedData();
		}, PTR.settleMs);
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
		return post.entry_date !== todayEntryDate().slice(0, 10) && post.entry_date < todayEntryDate();
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
>
	{#if pullVisible(pull)}
		<div class="ptr" style:height="{ptrHeight}px" aria-hidden="true">
			<div class="ptr-mark" style:height="{pullMarkHeight(pull)}px">
				<Mark />
			</div>
		</div>
	{/if}

	<div
		class="feed"
		role="feed"
		aria-label="Лента"
		bind:this={feedEl}
		ontouchstart={onTouchStart}
		ontouchmove={onTouchMove}
		ontouchend={onTouchEnd}
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
			<Hint style="margin:16px">{joinAvatarHint}</Hint>
		{/if}
		{#if !circle.canWrite}
			<Hint style="margin:16px">Вы читаете этот круг и не пишете.</Hint>
		{/if}
		{#if loading}
			<Hint style="margin:24px 16px">Загрузка…</Hint>
		{:else if error && !posts.length}
			<Hint style="margin:24px 16px">{error}</Hint>
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
							<Avatar
								initial={circle.identityInitial}
								color={circle.colorHex}
								src={circle.avatarUrl}
							/>
							<div>
								<div class="n">{circle.identityName}</div>
								<div class="tm" style="display:flex;align-items:center;gap:5px">
									<Icon name="clock" size="xs" />в очереди
								</div>
							</div>
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
					{#each splitMentionBody(post.body) as part}
						{#if part.kind === 'mention'}<span class="men">{part.value}</span>{:else}{part.value}{/if}
					{/each}
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
							{count}
							locationLabel={loc || undefined}
							onclick={() => openAlbum(post.id)}
						/>
					{/if}
					{#each attachmentMedia(post.media) as att (att.blob_id)}
						<AttachmentRow
							filename={attachmentLabel(att)}
							size={attachmentSizeLabel(att, formatBytes)}
							onclick={() => {
								void downloadBlob(circle.origin, att.blob_id, attachmentLabel(att));
							}}
						/>
					{/each}
				{/snippet}
				{#snippet postComments()}
					{@const preview = commentPreview(post.comments)}
					{#if preview.first}
						<div>
							{preview.first}{#if preview.createdAt}<span class="tm"
								> · {formatPostTime(preview.createdAt)}</span
							>{/if}
						</div>
					{/if}
					{#if preview.more}
						<div class="mo">ещё {plural(preview.more, WORD.comment)}</div>
					{/if}
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
						<Avatar
							initial={authorInitial(post.author_name)}
							color={circle.colorHex}
							src={authorAvatarSrc(post)}
						/>
						<div>
							<div class="n">{post.author_name}</div>
							<div class="tm">{formatPostTime(post.created_at)}</div>
						</div>
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
					<Hint style="margin:0 16px 12px">{archiveHint}</Hint>
				{/if}
				{#if post.comments?.length}
					<CommentPreview onclick={() => openPost(post.id)}>
						{#snippet children()}
							{@render postComments()}
						{/snippet}
					</CommentPreview>
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

			{#if error}
				<Hint style="margin:16px">{error}</Hint>
			{/if}

			{#if !posts.length && !queuedPosts.length}
				<div class="empty">
					<Mark />
					<div class="h1s ctr" style="margin-top:28px">Пока ничего</div>
					<!-- Читатель не пишет и не зовёт: призыв и «Пригласить» — только пишущим. -->
					{#if circle.canWrite}
						<Hint centered style="margin:8px 34px 0">
							Напишите первым — или позовите тех, с кем хотите это вести.
						</Hint>
						<Button
							variant="colored"
							style="margin:24px auto 0;width:min(280px,100%)"
							onclick={openInvite}
						>
							Пригласить
						</Button>
					{/if}
				</div>
			{:else if posts.length}
				{#if showVisibilityCutoff && visibleFrom}
					<div class="feed-end cutoff">
						<Mark />
						<div style="font-size:12.5px;margin-top:6px">Вы здесь с {formatIsoDay(visibleFrom)}</div>
						<Hint centered style="margin:8px 16px 0">что было раньше — не ваше</Hint>
						{#if circleStartedAt}
							<div class="sep" style="margin:20px 40px"></div>
							<Hint centered>круг живёт с {formatIsoDay(circleStartedAt, true)}</Hint>
						{/if}
					</div>
				{:else if circleStartedAt}
					<div class="feed-end start">
						<Mark />
						<Hint centered style="margin:10px 16px 0;color:var(--muted)">Здесь начинается круг</Hint>
						<div class="hint ctr" style="margin-top:4px;color:var(--muted)">
							{formatIsoDay(circleStartedAt)}
						</div>
					</div>
				{/if}
			{/if}
		{/if}
	</div>

	{#if reactionsPost}
		<OverlayLayout label="Реакции" ondismiss={closeReactions}>
			<SectionLabel style="margin-top:2px">
				Реакция · {reactionsPost.reactions?.length ?? 0}
			</SectionLabel>
			{#each reactionsPost.reactions ?? [] as rx (rx.id)}
				<ReactionListRow
					initial={authorInitial(rx.author_name)}
					name={rx.author_name}
					color={circle.colorHex}
					icon={reactionIconName(rx.emoji)}
				/>
			{/each}
			<Hint style="margin-top:14px">
				Реакция одна на человека и подчиняется окну правок. Хотите сказать больше — напишите словами.
			</Hint>
		</OverlayLayout>
	{/if}

	{#if queueToRemove !== null}
		<OverlayLayout variant="dialog" label="Удалить неотправленную запись?" ondismiss={() => (queueToRemove = null)}>
			<div style="font-size:17px;font-weight:600;margin-bottom:10px">
				Удалить неотправленную запись?
			</div>
			<Hint style="margin-bottom:4px">
				Она ещё не ушла на сервер. Удалить — значит потерять текст и снимки.
			</Hint>
			<div class="rowin" style="margin:18px 0 0">
				<Button variant="ghost" style="flex:1;margin:0" onclick={() => (queueToRemove = null)}>
					Оставить
				</Button>
				<Button style="flex:1;margin:0" onclick={() => void confirmQueueRemove()}>Удалить</Button>
			</div>
		</OverlayLayout>
	{/if}
</CircleLayout>
