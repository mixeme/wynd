<!-- Smoke-test: вложения комментария from docs/screens.html #e4-29, #e4-30 -->
<script lang="ts">
	import CommentMedia from '$ui/data/CommentMedia.svelte';
	import CommentPreview from '$ui/data/CommentPreview.svelte';
	import CommentRow from '$ui/data/CommentRow.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import Thread from '$ui/data/Thread.svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import type { CommentMediaItem } from '$lib/journal/present';

	const photos: CommentMediaItem[] = ['a', 'b', 'c', 'd', 'e'].map((key) => ({ key, kind: 'photo' }));
	const voice: CommentMediaItem[] = [
		{ key: 'v', kind: 'voice', durationMs: 48000, peaks: [10, 40, 80, 30, 60, 90, 20, 50, 70, 35] }
	];
	const file: CommentMediaItem[] = [
		{ key: 'f', kind: 'file', name: 'vedra-i-lestnica.pdf', size: 245760 }
	];
</script>

<CircleLayout
	title="Обсуждение"
	identity="Мышь"
	avatar="М"
	tabs={false}
	commentPlaceholder="Написать комментарий…"
	commentPending={[{ key: 'p', name: 'smeta.pdf' }]}
	onCommentRemovePending={() => {}}
	onCommentPhotos={() => {}}
	onCommentFiles={() => {}}
	onCommentSend={() => {}}
>
	<PostCard>
		{#snippet author()}
			<Avatar initial="А" color="#58673A" />
			<div>
				<div class="n">Аня</div>
				<div class="tm">сегодня, 14:02</div>
			</div>
		{/snippet}
		{#snippet text()}
			Были на даче, все живы.
		{/snippet}
	</PostCard>
	<CommentPreview first="Аня: вот столько" time="сегодня, 14:22" more="ещё 1 комментарий" onclick={() => {}}>
		{#snippet media()}
			<CommentMedia items={photos} onphoto={() => {}} />
		{/snippet}
	</CommentPreview>
	<Thread>
		<CommentRow initial="А" name="Аня" color="#58673A">
			{#snippet time()}14:22{/snippet}
			вот столько
			<CommentMedia items={photos} onphoto={() => {}} />
		</CommentRow>
		<CommentRow initial="К" name="Кот" color="#62452F">
			{#snippet time()}14:31{/snippet}
			<CommentMedia items={voice} />
		</CommentRow>
		<CommentRow initial="М" name="Мышь" onedit={() => {}} ondelete={() => {}}>
			{#snippet time()}16:40{/snippet}
			список, что везти в субботу
			<CommentMedia items={file} onfile={() => {}} />
		</CommentRow>
	</Thread>
</CircleLayout>
