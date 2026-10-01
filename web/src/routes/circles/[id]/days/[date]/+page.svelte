<script lang="ts">
	import PostByline from '$ui/data/PostByline.svelte';
	import EntryDateMark from '$ui/data/EntryDateMark.svelte';
	import PullRefreshBand from '$ui/data/PullRefresh.svelte';
	import { PullRefresh } from '$lib/gestures/pullRefresh.svelte';
	import MentionText from '$ui/data/MentionText.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onDestroy, onMount } from 'svelte';
	import DayHeader from '$ui/data/DayHeader.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Input from '$ui/forms/Input.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { isAccessError } from '$lib/api/client';
	import { formatEntryDate, formatPostTime, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { clearDayTitle, loadDay, loadDays, setDayTitle } from '$lib/journal/days';
	import { authorInitial, coverMedia, locationLabel, mediaCount, photoMedia,
		localDayOf
	} from '$lib/journal/present';
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
	// Название и обложку меняет тот, у кого есть запись за этот день
	// (wynd.html, «Дни»). Без своей записи сервер ответил бы forbidden —
	// поэтому ни подсказки «нажмите», ни входа в правку.
	const canEditDay = $derived(
		circle.canWrite && posts.some((p) => p.identity_id === circle.identityId)
	);
	const titleSubtitle = $derived(
		canEditDay ? `${formatEntryDate(entryDate)} · нажмите, чтобы изменить` : formatEntryDate(entryDate)
	);

	function isBackfilled(post: FeedPost): boolean {
		return post.entry_date === entryDate && localDayOf(post.created_at) > entryDate;
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

	// Обновление жестом (3.5), как в ленте: тянешь экран от верха.
	let listEl: HTMLDivElement | undefined = $state();
	const ptr = new PullRefresh(() => listEl?.scrollTop ?? 0, () => loadData());
	onDestroy(() => ptr.destroy());

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
	<PullRefreshBand pull={ptr.state} />
	<div
		class="feed"
		role="feed"
		aria-label="День"
		bind:this={listEl}
		ontouchstart={ptr.start}
		ontouchmove={ptr.move}
		ontouchend={ptr.end}
	>
	{#if loading}
		<Loading />
	{:else if error && !posts.length}
		<Hint class="gutter-24">{error}</Hint>
	{:else}
		<DayHeader
			coverUrl={coverUrl || undefined}
			title={editingTitle ? undefined : dayTitle}
			subtitle={editingTitle ? undefined : titleSubtitle}
			oncover={canEditDay ? openDayAlbum : undefined}
			ontitle={editingTitle || !canEditDay ? undefined : startEditTitle}
		/>

		{#if editingTitle}
			<Input class="mt-14" bind:value={titleDraft} active placeholder={formatEntryDate(entryDate)} />
			<div class="rowin mt-12">
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
				<Hint centered class="mt-8">
					<TextButton onclick={removeTitle}>убрать название</TextButton>
				</Hint>
			{/if}
		{/if}

		{#if canEditDay}
			<Hint class="gutter-12">
				День общий: название и обложку может сменить любой, у кого есть запись за этот день. Если
				поменяют несколько — останется последнее.
			</Hint>
		{:else if circle.canWrite}
			<Hint class="gutter-12">
				Название и обложку дня меняют те, у кого есть запись за этот день.
			</Hint>
		{/if}
		{#each posts as post (post.id)}
			{#snippet backfilled()}
				<EntryDateMark icon="clock" label="внесено сегодня" />
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
					<PostByline
						initial={authorInitial(post.author_name)}
						color={circle.colorHex}
						src={authorAvatarSrc(post)}
						name={post.author_name}
						time={formatPostTime(post.created_at, post.entry_date)}
					/>
				{/snippet}
			</PostCard>
		{/each}
		{#if !posts.length}
			<Hint class="gutter-24">В этот день записей нет</Hint>
		{/if}
		{#if error}
			<Hint class="gutter-12">{error}</Hint>
		{/if}
	{/if}
	</div>
</CircleLayout>

