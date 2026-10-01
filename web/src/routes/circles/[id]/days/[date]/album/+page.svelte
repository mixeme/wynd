<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatEntryDate, isEditableActive } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadDay, loadDays, setDayCover, clearDayCover } from '$lib/journal/days';
	import { photoMedia } from '$lib/journal/present';
	import { getMediaUrl } from '$lib/media/objectUrl';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const entryDate = $derived($page.params.date ?? '');

	let items = $state<
		Array<{ postId: string; blobId: string; kind: string; preview: string }>
	>([]);
	let selected = $state<{ postId: string; blobId: string } | undefined>();
	let coverBlobId = $state<string | undefined>();
	let coverPostId = $state<string | undefined>();
	let coverEditableUntil = $state<string | null | undefined>();
	let ownPostThatDay = $state(false);
	let loading = $state(true);
	let saving = $state(false);
	let clearing = $state(false);
	let error = $state('');

	const canClearCover = $derived(
		Boolean(coverPostId) && ownPostThatDay && isEditableActive(coverEditableUntil)
	);

	onMount(() => {
		// Альбом дня — выбор обложки; читателю он не нужен (SCR-2).
		if (!circle.canWrite) {
			goto(`/circles/${circle.circleId}/days/${entryDate}`, { replaceState: true });
			return;
		}
		void load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const [day, daysSnap] = await Promise.all([
				loadDay(circle.origin, circle.circleId, entryDate),
				loadDays(circle.origin, circle.circleId)
			]);
			const meta = daysSnap.days.find((d) => d.entry_date === entryDate);
			coverBlobId = meta?.cover_blob_id;
			coverPostId = meta?.cover_post_id;
			coverEditableUntil = meta?.cover_editable_until;
			ownPostThatDay = day.posts.some((p) => p.identity_id === circle.identityId);
			const next = [];
			for (const post of day.posts) {
				for (const m of photoMedia(post.media)) {
					next.push({
						postId: post.id,
						blobId: m.blob_id,
						kind: m.kind,
						preview: await getMediaUrl(circle.origin, m.blob_id)
					});
				}
			}
			items = next;
			const current = next.find((i) => i.blobId === coverBlobId) ?? next[0];
			if (current) selected = { postId: current.postId, blobId: current.blobId };
		} catch {
			error = 'Не удалось загрузить медиа дня';
		} finally {
			loading = false;
		}
	}

	function goBack() {
		goto(`/circles/${circle.circleId}/days/${entryDate}`);
	}

	function selectItem(item: (typeof items)[number]) {
		selected = { postId: item.postId, blobId: item.blobId };
	}

	async function save() {
		if (!selected) return;
		saving = true;
		error = '';
		try {
			await setDayCover(
				circle.origin,
				circle.circleId,
				entryDate,
				selected.postId,
				selected.blobId
			);
			goBack();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			saving = false;
		}
	}

	async function clearCover() {
		clearing = true;
		error = '';
		try {
			await clearDayCover(circle.origin, circle.circleId, entryDate);
			goBack();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			clearing = false;
		}
	}
</script>

<FormLayout app color={circle.color} title="Альбом" onback={goBack}>
	{#snippet bar()}
		<TextButton variant="bar" style="font-size:13.5px" onclick={goBack}>Отмена</TextButton>
		<TextButton
			variant="barAction"
			active={Boolean(selected) && !saving}
			disabled={!selected}
			loading={saving}
			onclick={() => save()}
		>
			Сохранить
		</TextButton>
	{/snippet}

	<Hint style="margin:0 16px 8px">{formatEntryDate(entryDate)}</Hint>

	{#if loading}
		<Loading />
	{:else if !items.length}
		<Hint class="gutter-24">В этот день нет фото или видео</Hint>
	{:else}
		<PhotoGrid style="padding:0 12px 16px">
			{#each items as item (item.blobId)}
				<MediaTile
					variant="album"
					src={item.preview}
					kind={item.kind === 'video' ? 'video' : 'photo'}
					selected={selected?.blobId === item.blobId}
					onclick={() => selectItem(item)}
				/>
			{/each}
		</PhotoGrid>
	{/if}
	{#if canClearCover}
		<Hint class="mt-12" centered>
			<TextButton disabled={clearing} onclick={() => void clearCover()}>убрать обложку</TextButton>
		</Hint>
	{/if}

	{#if error}
		<Hint class="gutter-12">{error}</Hint>
	{/if}
</FormLayout>

