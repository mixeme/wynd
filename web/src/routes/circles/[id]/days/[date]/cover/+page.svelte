<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadDay, setDayCover } from '$lib/journal/days';
	import { photoMedia } from '$lib/journal/present';
	import { getMediaUrl } from '$lib/media/objectUrl';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const entryDate = $derived($page.params.date ?? '');

	let items = $state<
		Array<{ postId: string; blobId: string; kind: string; preview: string; isCover: boolean }>
	>([]);
	let selected = $state<{ postId: string; blobId: string } | undefined>();
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');

	onMount(() => {
		void load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const day = await loadDay(circle.origin, circle.circleId, entryDate);
			const next = [];
			for (const post of day.posts) {
				for (const m of photoMedia(post.media)) {
					next.push({
						postId: post.id,
						blobId: m.blob_id,
						kind: m.kind,
						preview: await getMediaUrl(circle.origin, m.blob_id),
						isCover: m.is_cover
					});
				}
			}
			items = next;
			const current = next.find((i) => i.isCover) ?? next[0];
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
</script>

<FormLayout app color={circle.color} title="Обложка дня" onback={goBack}>
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

	{#if loading}
		<Hint style="margin:24px 16px">Загрузка…</Hint>
	{:else if !items.length}
		<Hint style="margin:24px 16px">В этот день нет фото или видео</Hint>
	{:else}
		<div class="g3" style="padding:12px">
			{#each items as item (item.blobId)}
				<button
					type="button"
					class="cell"
					class:on={selected?.blobId === item.blobId}
					onclick={() => selectItem(item)}
				>
					{#if item.kind === 'video'}
						<video src={item.preview} muted playsinline></video>
					{:else}
						<img src={item.preview} alt="" />
					{/if}
					{#if selected?.blobId === item.blobId}
						<span class="mark">✓</span>
					{/if}
				</button>
			{/each}
		</div>
	{/if}

	{#if error}
		<Hint style="margin:12px 16px">{error}</Hint>
	{/if}
</FormLayout>

<style>
	.g3 {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 4px;
	}
	.cell {
		position: relative;
		aspect-ratio: 1;
		overflow: hidden;
		border-radius: 4px;
		background: var(--tint);
		border: none;
		padding: 0;
		cursor: pointer;
	}
	.cell.on {
		outline: 2px solid var(--c);
		outline-offset: 1px;
	}
	.cell img,
	.cell video {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.mark {
		position: absolute;
		right: 4px;
		top: 4px;
		background: var(--c);
		color: #fff;
		border-radius: 50%;
		width: 20px;
		height: 20px;
		font-size: 12px;
		line-height: 20px;
	}
</style>
