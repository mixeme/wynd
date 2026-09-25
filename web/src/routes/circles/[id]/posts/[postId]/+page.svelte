<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import AttachmentRow from '$ui/data/AttachmentRow.svelte';
	import CommentRow from '$ui/data/CommentRow.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import ReactionListRow from '$ui/data/ReactionListRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchMembers, type MemberInfo } from '$lib/circles/settings';
	import { formatBytes } from '$lib/format/bytes';
	import { formatClock, formatPostTime, isEditableActive } from '$lib/format/time';
	import { isPostArchiveLocked } from '$lib/journal/archive';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import { splitMentionBody } from '$lib/journal/mentions';
	import {
		attachmentLabel,
		attachmentMedia,
		attachmentSizeLabel,
		authorInitial,
		coverMedia,
		findPost,
		groupReactions,
		ownReaction,
		reactionIconName,
		REACTION_KEYS
	} from '$lib/journal/present';
	import {
		createComment,
		deleteComment,
		editComment,
		removeReaction,
		setReaction
	} from '$lib/journal/posts';
	import type { Comment, FeedPost } from '$lib/journal/types';
	import { downloadBlob, getMediaUrl } from '$lib/media/objectUrl';
	import {
		enqueueComment,
		enqueueReaction,
		enqueueReactionRemove,
		listQueuedComments,
		subscribeQueue,
		type QueuedCommentView
	} from '$lib/queue/queue';
	import { isTransportError } from '$lib/queue/transport';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const postId = $derived($page.params.postId ?? '');

	let post = $state<FeedPost | undefined>();
	let coverUrl = $state('');
	let postAvatarUrl = $state('');
	let commentAvatarUrls = $state<Record<string, string>>({});
	let loading = $state(true);
	let error = $state('');
	let draft = $state('');
	let editingCommentId = $state('');
	let editingCommentBody = $state('');
	let activeMemberCount = $state(2);
	let members = $state<MemberInfo[]>([]);
	let queuedComments = $state<QueuedCommentView[]>([]);
	let pickerOpen = $state(false);

	const commentMembers = $derived(members.filter((m) => m.status === 'active'));

	const soloCircle = $derived(activeMemberCount === 1);
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

	function refreshQueued() {
		void listQueuedComments(circle.origin, circle.circleId, postId).then((items) => {
			queuedComments = items;
		});
	}

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
		const unsubSync = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['feed'],
			refetch: load
		});
		return () => {
			unsubQueue();
			unsubSync();
		};
	});

	async function load() {
		try {
			const snap = await loadFeed(circle.origin, circle.circleId);
			post = findPost(snap.posts, postId);
			const cover = post ? coverMedia(post.media) : undefined;
			if (cover) {
				coverUrl = await getMediaUrl(circle.origin, cover.blob_id);
			} else {
				coverUrl = '';
			}
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
		} catch {
			error = 'Не удалось загрузить запись';
		} finally {
			loading = false;
		}
	}

	function goBack() {
		goto(`/circles/${circle.circleId}`);
	}

	function openEdit() {
		goto(`/circles/${circle.circleId}/compose?post=${postId}`);
	}

	function openAlbum() {
		goto(`/circles/${circle.circleId}/posts/${postId}/album`);
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

	async function saveCommentEdit(commentId: string) {
		const text = editingCommentBody.trim();
		if (!text) return;
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
		if (postLocked) return;
		const text = draft.trim();
		if (!text) return;
		error = '';
		try {
			if (navigator.onLine) {
				await createComment(circle.origin, circle.circleId, postId, text);
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
		if (!circle.canWrite || soloCircle || postLocked) return false;
		const mine = ownReaction(currentPost.reactions, circle.identityId);
		if (!mine) return true;
		return isEditableActive(mine.editable_until);
	}

	function applyReaction(emoji: string | null) {
		if (!post) return;
		const reactions = [...(post.reactions ?? [])];
		const idx = reactions.findIndex((r) => r.identity_id === circle.identityId);
		if (emoji === null) {
			if (idx >= 0) reactions.splice(idx, 1);
		} else if (idx >= 0) {
			reactions[idx] = { ...reactions[idx], emoji };
		} else {
			reactions.push({
				id: `local-${post.id}`,
				post_id: post.id,
				emoji,
				author_name: circle.identityName,
				identity_id: circle.identityId,
				created_at: new Date().toISOString()
			});
		}
		post = { ...post, reactions };
	}

	async function pickReaction(currentPost: FeedPost, emoji: string) {
		if (postLocked) return;
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
	bind:commentDraft={draft}
	commentBar={circle.canWrite && !postLocked}
	onback={goBack}
	onCommentSend={circle.canWrite && !postLocked ? sendComment : undefined}
>
	{#if loading}
		<Hint style="margin:24px 16px">Загрузка…</Hint>
	{:else if !post}
		<Hint style="margin:24px 16px">{error || 'Запись не найдена'}</Hint>
	{:else}
		{@const currentPost = post}
		{@const cover = coverMedia(currentPost.media)}
		{#snippet postHeaderRight()}
			<div style="display:flex;align-items:center;gap:10px;margin-left:auto">
				{#if canEditPost(currentPost)}
					<IconButton name="edit" label="Править" size="sm" onclick={openEdit} />
				{/if}
				{#if coverUrl && cover}
					<MediaTile
						variant="headerMini"
						src={coverUrl}
						kind={cover.kind === 'video' ? 'video' : 'photo'}
						aria-label="Открыть альбом"
						onclick={openAlbum}
					/>
				{/if}
			</div>
		{/snippet}
		{#snippet postText()}
			{#each splitMentionBody(currentPost.body) as part (part.kind + part.value)}
				{#if part.kind === 'mention'}<span class="men">{part.value}</span>{:else}{part.value}{/if}
			{/each}
		{/snippet}
		{#snippet postMedia()}
			{#each attachmentMedia(currentPost.media) as att (att.blob_id)}
				<AttachmentRow
					filename={attachmentLabel(att)}
					size={attachmentSizeLabel(att, formatBytes)}
					onclick={() => {
						void downloadBlob(circle.origin, att.blob_id, attachmentLabel(att));
					}}
				/>
			{/each}
		{/snippet}
		<PostCard
			style="margin-top:8px"
			headerRight={postHeaderRight}
			text={currentPost.body ? postText : undefined}
			media={attachmentMedia(currentPost.media).length ? postMedia : undefined}
		>
			{#snippet author()}
				<Avatar
					initial={authorInitial(currentPost.author_name)}
					color={circle.colorHex}
					src={currentPost.identity_id === circle.identityId && circle.avatarUrl
						? circle.avatarUrl
						: postAvatarUrl || undefined}
				/>
				<div>
					<div class="n">{currentPost.author_name}</div>
					<div class="tm">{formatPostTime(currentPost.created_at, currentPost.entry_date)}</div>
				</div>
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
			<Hint style="margin:0 16px 12px">{archiveHint}</Hint>
		{/if}

		<div class="thread">
			{#each currentPost.comments ?? [] as comment (comment.id)}
				<CommentRow
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
							<TextArea variant="field" class="ced" bind:value={editingCommentBody} />
							<div class="rowin">
								<Button
									variant="colored"
									style="flex:1;margin:0"
									onclick={() => saveCommentEdit(comment.id)}
								>
									Сохранить
								</Button>
								<Button variant="ghost" style="flex:1;margin:0" onclick={cancelEditComment}>
									Отмена
								</Button>
							</div>
						{:else}
							{#each splitMentionBody(comment.body) as part (part.kind + part.value)}
								{#if part.kind === 'mention'}<span class="men">{part.value}</span>{:else}{part.value}{/if}
							{/each}
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
						<span style="display:flex;align-items:center;gap:5px">
							<Icon name="clock" size="xs" />в очереди
						</span>
					{/snippet}
					{#snippet children()}
						{#each splitMentionBody(item.body) as part (part.kind + part.value)}
							{#if part.kind === 'mention'}<span class="men">{part.value}</span>{:else}{part.value}{/if}
						{/each}
						{#if item.state === 'failed' && item.error}
							<Hint style="margin-top:8px">{item.error}</Hint>
						{/if}
					{/snippet}
				</CommentRow>
			{/each}
		</div>

		{#if error}
			<Hint style="margin:0 16px 16px">{error}</Hint>
		{/if}
	{/if}

	{#if reactionsOpen && post}
		<OverlayLayout ondismiss={closeReactions}>
			<SectionLabel style="margin-top:2px">
				Реакция · {post.reactions?.length ?? 0}
			</SectionLabel>
			{#each post.reactions ?? [] as rx (rx.id)}
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

