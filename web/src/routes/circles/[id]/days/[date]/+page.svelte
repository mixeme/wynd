<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Icon from '$ui/Icon.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { isAccessError } from '$lib/api/client';
	import { formatEntryDate, formatPostTime, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import {
		clearDayCover,
		clearDayTitle,
		loadDay,
		loadDays,
		setDayTitle
	} from '$lib/journal/days';
	import { authorInitial, coverMedia, mediaCount, photoMedia } from '$lib/journal/present';
	import type { FeedPost } from '$lib/journal/types';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const entryDate = $derived($page.params.date ?? '');

	let posts = $state<FeedPost[]>([]);
	let dayTitle = $state('');
	let titleDraft = $state('');
	let titleEditableUntil = $state<string | null | undefined>();
	let coverEditableUntil = $state<string | null | undefined>();
	let hasCustomTitle = $state(false);
	let coverBlobId = $state<string | undefined>();
	let loading = $state(true);
	let savingTitle = $state(false);
	let error = $state('');
	let coverUrl = $state('');
	let mediaUrls = $state<Record<string, string>>({});

	const canClearTitle = $derived(hasCustomTitle && isEditableActive(titleEditableUntil));
	const canClearCover = $derived(Boolean(coverEditableUntil) && isEditableActive(coverEditableUntil));

	function todayEntryDate(): string {
		const d = new Date();
		return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	}

	function isBackfilled(post: FeedPost): boolean {
		return post.entry_date === entryDate && post.created_at.slice(0, 10) > entryDate;
	}

	async function resolveMedia(feedPosts: FeedPost[]) {
		const next: Record<string, string> = { ...mediaUrls };
		for (const post of feedPosts) {
			for (const m of photoMedia(post.media)) {
				if (!next[m.blob_id]) {
					try {
						next[m.blob_id] = await getMediaUrl(circle.origin, m.blob_id);
					} catch {
						/* skip */
					}
				}
			}
		}
		mediaUrls = next;
	}

	async function loadData() {
		error = '';
		try {
			const [daySnap, daysSnap] = await Promise.all([
				loadDay(circle.origin, circle.circleId, entryDate),
				loadDays(circle.origin, circle.circleId)
			]);
			posts = daySnap.posts;
			const meta = daysSnap.days.find((d) => d.entry_date === entryDate);
			hasCustomTitle = Boolean(meta?.title);
			dayTitle = meta?.title || formatEntryDate(entryDate);
			titleDraft = meta?.title ?? '';
			titleEditableUntil = meta?.title_editable_until;
			coverEditableUntil = meta?.cover_editable_until;
			const blobId = meta?.cover_blob_id;
			if (blobId) {
				coverBlobId = blobId;
			} else {
				const first = posts.find((p) => coverMedia(p.media));
				coverBlobId = first ? coverMedia(first.media)?.blob_id : undefined;
			}
			if (coverBlobId) {
				try {
					coverUrl = await getMediaUrl(circle.origin, coverBlobId);
				} catch {
					coverUrl = '';
				}
			} else {
				coverUrl = '';
			}
			await resolveMedia(posts);
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить день';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void loadData();
		const unsub = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['day', 'days'],
			refetch: loadData
		});
		return unsub;
	});

	function goBack() {
		goto(`/circles/${circle.circleId}/days`);
	}

	function openPost(postId: string) {
		goto(`/circles/${circle.circleId}/posts/${postId}`);
	}

	function openAlbum(postId: string) {
		goto(`/circles/${circle.circleId}/posts/${postId}/album`);
	}

	function openCoverPicker() {
		goto(`/circles/${circle.circleId}/days/${entryDate}/cover`);
	}

	async function saveTitle() {
		const title = titleDraft.trim();
		if (!title) return;
		savingTitle = true;
		error = '';
		try {
			await setDayTitle(circle.origin, circle.circleId, entryDate, title);
			await loadData();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			savingTitle = false;
		}
	}

	async function removeTitle() {
		error = '';
		try {
			await clearDayTitle(circle.origin, circle.circleId, entryDate);
			await loadData();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function removeCover() {
		error = '';
		try {
			await clearDayCover(circle.origin, circle.circleId, entryDate);
			await loadData();
		} catch (err) {
			error = authErrorHint(err);
		}
	}
</script>

<CircleLayout
	app
	color={circle.color}
	title={dayTitle}
	identity={formatEntryDate(entryDate)}
	tabs={false}
	commentBar={false}
	onback={goBack}
>
	{#if loading}
		<Hint style="margin:24px 16px">Загрузка…</Hint>
	{:else if error && !posts.length}
		<Hint style="margin:24px 16px">{error}</Hint>
	{:else}
		{#if coverUrl}
			<button type="button" class="pic cover-wrap" onclick={openCoverPicker}>
				<img src={coverUrl} alt="" />
				<span class="tagr">обложка дня</span>
				<span class="cnt">сменить</span>
			</button>
		{:else}
			<button type="button" class="cover-empty" onclick={openCoverPicker}>Выбрать обложку</button>
		{/if}

		<div class="title-row">
			<Input bind:value={titleDraft} placeholder={formatEntryDate(entryDate)} />
			<Button
				variant="colored"
				disabled={savingTitle || !titleDraft.trim()}
				loading={savingTitle}
				onclick={saveTitle}
			>
				Сохранить
			</Button>
		</div>
		{#if canClearTitle || canClearCover}
			<div class="remove-actions">
				{#if canClearTitle}
					<Button variant="ghost" onclick={removeTitle}>убрать название</Button>
				{/if}
				{#if canClearCover}
					<Button variant="ghost" onclick={removeCover}>убрать обложку</Button>
				{/if}
			</div>
		{/if}

		<Hint style="margin:12px 16px">
			День общий: название и обложку может сменить любой, у кого есть запись за этот день. Если
			поменяют несколько — останется последнее.
		</Hint>
		{#each posts as post (post.id)}
			{#snippet backfilled()}
				<span class="tm" style="display:flex;align-items:center;gap:5px">
					<Icon name="clock" size="xs" />внесено сегодня
				</span>
			{/snippet}
			{#snippet postText()}
				{post.body}
			{/snippet}
			{#snippet postMedia()}
				{@const cover = coverMedia(post.media)}
				{@const count = mediaCount(post.media)}
				<div
					class="pic sq"
					role="presentation"
					onclick={(e) => {
						e.stopPropagation();
						openAlbum(post.id);
					}}
				>
					{#if cover && mediaUrls[cover.blob_id]}
						{#if cover.kind === 'video'}
							<video src={mediaUrls[cover.blob_id]} muted playsinline></video>
						{:else}
							<img src={mediaUrls[cover.blob_id]} alt="" />
						{/if}
					{/if}
					{#if count > 1}
						<span class="cnt">{count}</span>
					{/if}
				</div>
			{/snippet}
			<PostCard
				onclick={() => openPost(post.id)}
				headerRight={isBackfilled(post) ? backfilled : undefined}
				text={post.body ? postText : undefined}
				media={coverMedia(post.media) ? postMedia : undefined}
			>
					{#snippet author()}
						<Avatar initial={authorInitial(post.author_name)} color={circle.colorHex} />
						<div>
							<div class="n">{post.author_name}</div>
							<div class="tm">{formatPostTime(post.created_at, post.entry_date)}</div>
						</div>
					{/snippet}
				</PostCard>
		{/each}
		{#if !posts.length}
			<Hint style="margin:24px 16px">В этот день записей нет</Hint>
		{/if}
		{#if error}
			<Hint style="margin:12px 16px">{error}</Hint>
		{/if}
	{/if}
</CircleLayout>

<style>
	.cover-wrap {
		position: relative;
		aspect-ratio: 16 / 9;
		border-radius: 0;
		border: 0;
		border-bottom: 1px solid var(--line);
		cursor: pointer;
		padding: 0;
		display: block;
		width: 100%;
		background: transparent;
	}
	.cover-wrap img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.cover-empty {
		display: block;
		width: calc(100% - 32px);
		margin: 12px 16px;
		padding: 12px;
		border: 1px dashed var(--line);
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
	}
	.title-row {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin: 12px 16px 0;
	}
	.title-row :global(.fld) {
		margin: 0;
		width: 100%;
	}
	.title-row :global(.btn) {
		margin: 0;
		width: 100%;
	}
	.remove-actions {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin: 8px 16px;
	}
	.remove-actions :global(.btn) {
		margin: 0;
		width: 100%;
	}
	.pic img,
	.pic video {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
</style>
