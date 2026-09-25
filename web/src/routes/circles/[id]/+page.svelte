<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onDestroy, onMount } from 'svelte';
	import ArchiveBanner from '$ui/data/ArchiveBanner.svelte';
	import AttachmentRow from '$ui/data/AttachmentRow.svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import EntryDateMark from '$ui/data/EntryDateMark.svelte';
	import EventDivider from '$ui/data/EventDivider.svelte';
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
	import { formatBytes } from '$lib/format/bytes';
	import { formatDeadline, formatEntryDate, formatPostTime, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import { splitMentionBody } from '$lib/journal/mentions';
	import { fetchMembers } from '$lib/circles/settings';
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
	import { downloadBlob, getMediaUrl } from '$lib/media/objectUrl';
	import {
		enqueuePost,
		enqueueReaction,
		enqueueReactionRemove,
		listQueuedPosts,
		removeQueueItem,
		subscribeQueue,
		type QueuedPostView
	} from '$lib/queue/queue';
	import { registerRefetch } from '$lib/sync/sync';
	import { authErrorHint } from '$lib/auth/auth';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let posts = $state<FeedPost[]>([]);
	let feedEvents = $state<FeedEvent[]>([]);
	let visibleFrom = $state<string | null>(null);
	let circleStartedAt = $state('');
	let activeMemberCount = $state(2);
	let queuedPosts = $state<QueuedPostView[]>([]);
	let loading = $state(true);
	let error = $state('');
	let joinAvatarHint = $state('');
	let dividerAt = $state<number | null>(null);
	let fixedLastRead = $state(0);
	let feedEl: HTMLDivElement | undefined = $state();
	let pullY = $state(0);
	let refreshing = $state(false);
	let ptrAnimating = $state(false);
	let commentDraft = $state('');
	let dayPromptDate = $state('');
	let pickerPostId = $state('');

	const reactionsPostId = $derived($page.url.searchParams.get('reactions'));
	const reactionsPost = $derived(
		reactionsPostId ? posts.find((p) => p.id === reactionsPostId) : undefined
	);

	let mediaUrls = $state<Record<string, string>>({});

	async function resolveMediaUrls(feedPosts: FeedPost[]) {
		const next: Record<string, string> = { ...mediaUrls };
		for (const post of feedPosts) {
			const cover = coverMedia(post.media);
			if (cover && !next[cover.blob_id]) {
				try {
					next[cover.blob_id] = await getMediaUrl(circle.origin, cover.blob_id);
				} catch {
					/* skip */
				}
			}
		}
		mediaUrls = next;
	}

	const soloCircle = $derived(activeMemberCount === 1);
	const showVisibilityCutoff = $derived(Boolean(visibleFrom));
	const ptrHeight = $derived(refreshing || ptrAnimating ? 72 : pullY);

	async function loadFeedData() {
		error = '';
		try {
			const snap = await loadFeed(circle.origin, circle.circleId);
			posts = snap.posts;
			feedEvents = snap.events ?? [];
			visibleFrom = snap.visible_from ?? null;
			circleStartedAt = snap.circle_started_at ?? '';
			dividerAt = unreadDividerIndex(posts, fixedLastRead);
			await resolveMediaUrls(posts);
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить ленту';
		} finally {
			loading = false;
			refreshing = false;
			ptrAnimating = false;
			pullY = 0;
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
			const url = new URL($page.url);
			url.searchParams.delete('joinAvatar');
			const next = `${url.pathname}${url.search}${url.hash}`;
			history.replaceState(history.state, '', next);
		}
		fixedLastRead = circle.lastReadSeq;
		const dayPromptParam = $page.url.searchParams.get('dayPrompt');
		void loadFeedData().then(() => {
			if (!dayPromptParam || !/^\d{4}-\d{2}-\d{2}$/.test(dayPromptParam)) return;
			const url = new URL($page.url);
			url.searchParams.delete('dayPrompt');
			const next = `${url.pathname}${url.search}${url.hash}`;
			history.replaceState(history.state, '', next);
			void checkDayPrompt(dayPromptParam);
		});
		void fetchMembers(circle.origin, circle.circleId)
			.then((list) => {
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
		if (!text) return;
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
			error = authErrorHint(err);
		}
	}

	function openComposeFromBar() {
		sessionStorage.setItem(`wynd.compose.${circle.circleId}`, commentDraft);
		goto(`/circles/${circle.circleId}/compose`);
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

	function openAlbum(postId: string) {
		goto(`/circles/${circle.circleId}/posts/${postId}/album`);
	}

	function openReactions(postId: string) {
		goto(`/circles/${circle.circleId}?reactions=${postId}`);
	}

	function closeReactions() {
		goto(`/circles/${circle.circleId}`);
	}

	function togglePicker(postId: string) {
		pickerPostId = pickerPostId === postId ? '' : postId;
	}

	function showReactionPlus(post: FeedPost): boolean {
		if (soloCircle) return false;
		const mine = ownReaction(post.reactions, circle.identityId);
		if (!mine) return true;
		return isEditableActive(mine.editable_until);
	}

	function applyReaction(postId: string, emoji: string | null) {
		posts = posts.map((p) => {
			if (p.id !== postId) return p;
			const reactions = [...(p.reactions ?? [])];
			const idx = reactions.findIndex((r) => r.identity_id === circle.identityId);
			if (emoji === null) {
				if (idx >= 0) reactions.splice(idx, 1);
			} else if (idx >= 0) {
				reactions[idx] = { ...reactions[idx], emoji };
			} else {
				reactions.push({
					id: `local-${postId}`,
					post_id: postId,
					emoji,
					author_name: circle.identityName,
					identity_id: circle.identityId,
					created_at: new Date().toISOString()
				});
			}
			return { ...p, reactions };
		});
	}

	async function pickReaction(post: FeedPost, emoji: string) {
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
		}
	}

	function onTouchStart(e: TouchEvent) {
		if (!feedEl || feedEl.scrollTop > 0) return;
		const y = e.touches[0]?.clientY ?? 0;
		feedEl.dataset.pullStart = String(y);
	}

	function onTouchMove(e: TouchEvent) {
		if (!feedEl || feedEl.scrollTop > 0) return;
		const start = Number(feedEl.dataset.pullStart ?? 0);
		const y = e.touches[0]?.clientY ?? 0;
		pullY = Math.max(0, Math.min(80, y - start));
	}

	function onTouchEnd() {
		if (pullY > 48 && !refreshing && !ptrAnimating) {
			ptrAnimating = true;
			pullY = 69;
			window.setTimeout(() => {
				refreshing = true;
				void loadFeedData();
			}, 280);
			return;
		}
		pullY = 0;
	}

	function formatIsoDay(iso: string): string {
		return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' }).format(
			new Date(iso)
		);
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
	onback={goBack}
	onsearch={openSearch}
	bind:commentDraft
	onCommentSend={sendFromBar}
	onCommentCompose={openComposeFromBar}
>
	{#if pullY > 0 || refreshing || ptrAnimating}
		<div class="ptr" style:height="{ptrHeight}px" aria-hidden="true">
			<div
				class="ptr-mark"
				style:height="{refreshing || ptrAnimating ? 69 : Math.max(8, pullY * 0.85)}px"
			>
				<Mark />
			</div>
		</div>
	{/if}

	<div
		class="feed"
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
		{#if loading}
			<Hint style="margin:24px 16px">Загрузка…</Hint>
		{:else if error && !posts.length}
			<Hint style="margin:24px 16px">{error}</Hint>
		{:else}
			{#each queuedPosts as item (item.id)}
				<!-- Snippets live outside <PostCard>: a {#snippet} nested in {#if} is not passed as a prop. -->
				{#snippet queuedMedia()}
					<div class="pic sq" style="background:var(--tint)">
						<span class="cnt">{item.file_count}</span>
					</div>
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
								name="dots"
								size="sm"
								label="Удалить из очереди"
								stopPropagation
								onclick={() => void removeQueueItem(item.id)}
							/>
						{/snippet}
						{#snippet text()}
							{item.body || 'Без текста'}
						{/snippet}
					</PostCard>
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
					{#each splitMentionBody(post.body) as part (part.kind + part.value)}
						{#if part.kind === 'mention'}<span class="men">{part.value}</span>{:else}{part.value}{/if}
					{/each}
				{/snippet}
				{#snippet postMedia()}
					{@const cover = coverMedia(post.media)}
					{@const count = mediaCount(post.media)}
					{@const loc = locationLabel(cover)}
					{#if cover}
						<div
							class="pic sq"
							role="presentation"
							onclick={(e) => {
								e.stopPropagation();
								openAlbum(post.id);
							}}
						>
							{#if mediaUrls[cover.blob_id]}
								<img src={mediaUrls[cover.blob_id]} alt="" />
							{/if}
							{#if loc}
								<span class="tagr"><Icon name="loc" size="xs" />{loc}</span>
							{/if}
							{#if count > 1}
								<span class="cnt">{count}</span>
							{/if}
						</div>
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
						<div>{preview.first}</div>
					{/if}
					{#if preview.more}
						<div class="mo">ещё {preview.more} комментариев</div>
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
							src={post.identity_id === circle.identityId ? circle.avatarUrl : undefined}
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
				{#if post.comments?.length}
					<CommentPreview onclick={() => openPost(post.id)}>
						{#snippet children()}
							{@render postComments()}
						{/snippet}
					</CommentPreview>
				{/if}
				{#if dayPromptDate && post.entry_date === dayPromptDate && i === posts.findIndex((p) => p.entry_date === dayPromptDate)}
					<div class="post day-prompt">
						<div style="font-weight:600">Первая запись за {formatEntryDate(dayPromptDate)}</div>
						<div style="margin-top:4px;color:var(--muted)">
							Дать этому дню название и обложку? День общий: увидят все, и поправить сможет любой, у
							кого есть за него запись.
						</div>
						<div class="rowin" style="margin:14px 0 0">
							<Button variant="colored" style="flex:1;margin:0" onclick={openDayFromPrompt}>
								Назвать день
							</Button>
							<Button variant="ghost" style="flex:1;margin:0" onclick={dismissDayPrompt}>
								Потом
							</Button>
						</div>
					</div>
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
					<Hint centered style="margin:8px 34px 0">
						Напиши первым — или позови тех, с кем хочешь это вести.
					</Hint>
					<Button
						variant="colored"
						style="margin:24px auto 0;width:min(280px,100%)"
						onclick={openInvite}
					>
						Пригласить
					</Button>
				</div>
			{:else if posts.length}
				{#if showVisibilityCutoff && visibleFrom}
					<div class="feed-end cutoff">
						<Mark />
						<div style="font-size:12.5px;margin-top:6px">Ты здесь с {formatIsoDay(visibleFrom)}</div>
						<Hint centered style="margin:8px 16px 0">что было раньше — не твоё</Hint>
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
		<OverlayLayout ondismiss={closeReactions}>
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
</CircleLayout>

<style>
	.feed {
		flex: 1;
		min-height: 0;
		overflow-x: hidden;
		overflow-y: auto;
		padding-bottom: 8px;
	}
	.ptr {
		display: flex;
		align-items: flex-start;
		justify-content: center;
		color: var(--muted);
		overflow: hidden;
	}
	.ptr-mark {
		overflow: hidden;
		width: var(--mark-w);
	}
	.ptr-mark :global(svg) {
		width: var(--mark-w);
		height: var(--mark-h);
	}
	.empty {
		text-align: center;
		color: var(--empty-ink);
		margin-top: 80px;
	}
	.empty :global(.mk) {
		width: 104px;
		height: 156px;
		margin: 0 auto;
	}
	.pic img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: inherit;
	}
	.day-prompt {
		border-style: dashed;
	}
	.feed-end {
		text-align: center;
		margin: 32px 16px 24px;
		color: var(--c);
	}
	.feed-end.start {
		color: var(--muted);
	}
	.feed-end :global(.mk) {
		width: var(--mark-w);
		height: var(--mark-h);
		margin: 0 auto;
		opacity: 0.45;
	}
	.feed-end.cutoff :global(.mk) {
		opacity: 1;
	}
</style>
