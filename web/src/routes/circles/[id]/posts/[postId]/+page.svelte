<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import AttachmentRow from '$ui/data/AttachmentRow.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatBytes } from '$lib/format/bytes';
	import { formatClock, formatPostTime, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadFeed } from '$lib/journal/feed';
	import {
		attachmentLabel,
		attachmentMedia,
		attachmentSizeLabel,
		authorInitial,
		coverMedia,
		findPost
	} from '$lib/journal/present';
	import { createComment, deleteComment, editComment } from '$lib/journal/posts';
	import type { Comment, FeedPost } from '$lib/journal/types';
	import { downloadBlob, getMediaUrl } from '$lib/media/objectUrl';
	import { enqueueComment } from '$lib/queue/queue';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const postId = $derived($page.params.postId ?? '');

	let post = $state<FeedPost | undefined>();
	let coverUrl = $state('');
	let loading = $state(true);
	let error = $state('');
	let draft = $state('');
	let editingCommentId = $state('');
	let editingCommentBody = $state('');

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
			const cover = post ? coverMedia(post.media) : undefined;
			if (cover) {
				coverUrl = await getMediaUrl(circle.origin, cover.blob_id);
			} else {
				coverUrl = '';
			}
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
		return comment.identity_id === circle.identityId && isEditableActive(comment.editable_until);
	}

	function canEditPost(currentPost: FeedPost): boolean {
		return currentPost.identity_id === circle.identityId && isEditableActive(currentPost.editable_until);
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
			}
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	function commentAvatarSrc(comment: Comment): string | undefined {
		return comment.identity_id === circle.identityId ? circle.avatarUrl : undefined;
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
	commentPlaceholder="Написать комментарий…"
	bind:commentDraft={draft}
	onback={goBack}
	onCommentSend={sendComment}
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
					<TextButton onclick={openEdit} style="font-size:12.5px">править</TextButton>
				{/if}
				{#if coverUrl && cover}
					<button type="button" class="pic sq mini" onclick={openAlbum}>
						{#if cover.kind === 'video'}
							<video src={coverUrl} muted playsinline></video>
						{:else}
							<img src={coverUrl} alt="" />
						{/if}
					</button>
				{/if}
			</div>
		{/snippet}
		{#snippet postText()}
			{currentPost.body}
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
					src={currentPost.identity_id === circle.identityId ? circle.avatarUrl : undefined}
				/>
				<div>
					<div class="n">{currentPost.author_name}</div>
					<div class="tm">{formatPostTime(currentPost.created_at, currentPost.entry_date)}</div>
				</div>
			{/snippet}
		</PostCard>

		<div class="thread">
			{#each currentPost.comments ?? [] as comment (comment.id)}
				<div class="cmt">
					<Avatar
						initial={authorInitial(comment.author_name)}
						color={circle.colorHex}
						src={commentAvatarSrc(comment)}
					/>
					<div class="g">
						{#if editingCommentId === comment.id}
							<TextArea
								variant="field"
								class="ced"
								bind:value={editingCommentBody}
							/>
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
							<div class="who">
								<b>{comment.author_name}</b>
								<span class="tm">{formatClock(comment.created_at)}</span>
							</div>
							{comment.body}
						{/if}
					</div>
					<div class="acts">
						{#if canEditComment(comment)}
							<IconButton
								name="edit"
								label="Править"
								size="sm"
								onclick={() => startEditComment(comment)}
							/>
							<IconButton
								name="trash"
								label="Удалить"
								size="sm"
								onclick={() => removeComment(comment.id)}
							/>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		{#if error}
			<Hint style="margin:0 16px 16px">{error}</Hint>
		{/if}
	{/if}
</CircleLayout>

<style>
	.pic.mini {
		width: 44px;
		height: 44px;
		border: none;
		padding: 0;
		cursor: pointer;
		border-radius: 8px;
	}
	.pic img,
	.pic video {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: inherit;
	}
</style>
