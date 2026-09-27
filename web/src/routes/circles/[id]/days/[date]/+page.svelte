<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import DayHeader from '$ui/data/DayHeader.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Input from '$ui/forms/Input.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Icon from '$ui/Icon.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { isAccessError } from '$lib/api/client';
	import { formatEntryDate, formatPostTime, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { splitMentionBody } from '$lib/journal/mentions';
	import { clearDayTitle, loadDay, loadDays, setDayTitle } from '$lib/journal/days';
	import { authorInitial, coverMedia, locationLabel, mediaCount, photoMedia } from '$lib/journal/present';
	import type { FeedPost } from '$lib/journal/types';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { registerRefetch } from '$lib/sync/sync';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const entryDate = $derived($page.params.date ?? '');

	let posts = $state<FeedPost[]>([]);
	let dayTitle = $state('');
	let titleDraft = $state('');
	let titleEditableUntil = $state<string | null | undefined>();
	let hasCustomTitle = $state(false);
	let coverBlobId = $state<string | undefined>();
	let loading = $state(true);
	let savingTitle = $state(false);
	let editingTitle = $state(false);
	let error = $state('');
	let coverUrl = $state('');
	let mediaUrls = $state<Record<string, string>>({});
	let authorAvatarUrls = $state<Record<string, string>>({});

	const canClearTitle = $derived(hasCustomTitle && isEditableActive(titleEditableUntil));
	// Вышедший с доступом читает, но не пишет: подсказка и правка названия ему
	// не показываются — сервер ответил бы forbidden (план 42, SCR-2).
	const titleSubtitle = $derived(
		circle.canWrite
			? `${formatEntryDate(entryDate)} · нажмите, чтобы изменить`
			: formatEntryDate(entryDate)
	);

	function isBackfilled(post: FeedPost): boolean {
		return post.entry_date === entryDate && post.created_at.slice(0, 10) > entryDate;
	}

	function authorAvatarSrc(post: FeedPost): string | undefined {
		if (post.identity_id === circle.identityId && circle.avatarUrl) return circle.avatarUrl;
		return authorAvatarUrls[post.identity_id];
	}

	async function resolveMedia(feedPosts: FeedPost[]) {
		const next: Record<string, string> = { ...mediaUrls };
		const avatars: Record<string, string> = { ...authorAvatarUrls };
		for (const post of feedPosts) {
			const avatarBlob = post.author_avatar_blob_id;
			if (avatarBlob && !avatars[post.identity_id]) {
				try {
					avatars[post.identity_id] = await getMediaUrl(circle.origin, avatarBlob);
				} catch {
					/* skip */
				}
			}
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
		authorAvatarUrls = avatars;
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

	function openDayAlbum() {
		goto(`/circles/${circle.circleId}/days/${entryDate}/album`);
	}

	function startEditTitle() {
		titleDraft = hasCustomTitle ? dayTitle : '';
		editingTitle = true;
	}

	function cancelEditTitle() {
		titleDraft = hasCustomTitle ? dayTitle : '';
		editingTitle = false;
	}

	async function saveTitle() {
		const title = titleDraft.trim();
		if (!title) return;
		savingTitle = true;
		error = '';
		try {
			await setDayTitle(circle.origin, circle.circleId, entryDate, title);
			editingTitle = false;
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
			editingTitle = false;
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
		<Loading />
	{:else if error && !posts.length}
		<Hint style="margin:24px 16px">{error}</Hint>
	{:else}
		<DayHeader
			coverUrl={coverUrl || undefined}
			title={editingTitle ? undefined : dayTitle}
			subtitle={editingTitle ? undefined : titleSubtitle}
			oncover={circle.canWrite ? openDayAlbum : undefined}
			ontitle={editingTitle || !circle.canWrite ? undefined : startEditTitle}
		/>

		{#if editingTitle}
			<Input bind:value={titleDraft} active placeholder={formatEntryDate(entryDate)} style="margin-top:14px" />
			<div class="rowin" style="margin-top:12px">
				<Button
					variant="colored"
					style="flex:1"
					disabled={savingTitle || !titleDraft.trim()}
					loading={savingTitle}
					onclick={saveTitle}
				>
					Сохранить
				</Button>
				<Button variant="ghost" style="flex:1;margin:0" onclick={cancelEditTitle}>Отмена</Button>
			</div>
			{#if canClearTitle}
				<div class="hint ctr" style="margin-top:8px">
					<TextButton onclick={removeTitle}>убрать название</TextButton>
				</div>
			{/if}
		{/if}

		{#if circle.canWrite}
			<Hint style="margin:12px 16px">
				День общий: название и обложку может сменить любой, у кого есть запись за этот день. Если
				поменяют несколько — останется последнее.
			</Hint>
		{/if}
		{#each posts as post (post.id)}
			{#snippet backfilled()}
				<span class="tm" style="display:flex;align-items:center;gap:5px">
					<Icon name="clock" size="xs" />внесено сегодня
				</span>
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
			{/snippet}
			<PostCard
				onclick={() => openPost(post.id)}
				headerRight={isBackfilled(post) ? backfilled : undefined}
				text={post.body ? postText : undefined}
				media={coverMedia(post.media) ? postMedia : undefined}
			>
				{#snippet author()}
					<Avatar
						initial={authorInitial(post.author_name)}
						color={circle.colorHex}
						src={authorAvatarSrc(post)}
					/>
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

