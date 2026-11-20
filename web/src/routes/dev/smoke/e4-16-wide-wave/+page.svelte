<script lang="ts">
	import { page } from '$app/state';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import PostByline from '$ui/data/PostByline.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import VoiceRow from '$ui/data/VoiceRow.svelte';

	const wide = $derived(page.url.searchParams.has('wide'));
	const examples = [
		{
			durationMs: 4200,
			time: 'сегодня, 10:46',
			peaks: [
				4, 4, 8, 8, 12, 18, 45, 64, 53, 70, 48, 66, 78, 44, 55, 37, 45, 71, 64, 50, 34, 65, 45, 62,
				43, 55, 41, 63, 43, 36, 52, 42, 31, 57, 39, 51, 27, 12, 8, 4
			]
		},
		{ durationMs: 200, time: 'сегодня, 10:40', peaks: [8, 11] }
	];

	function displayPeaks(peaks: number[]) {
		const minBars = 48;
		if (!wide || peaks.length >= minBars) return peaks;
		return Array.from(
			{ length: minBars },
			(_, index) => peaks[Math.floor((index * peaks.length) / minBars)]
		);
	}
</script>

<div class="preview">
	<CircleLayout
		app
		color="olive"
		title="Семья"
		identity="Аня"
		avatar="А"
		onback={() => {}}
		onsearch={() => {}}
		onCommentSend={() => {}}
		onCommentCompose={() => {}}
		onCommentPhotos={() => {}}
		onCommentVoice={() => {}}
		onCommentVideo={() => {}}
	>
		<div class="feed">
			{#each examples as { durationMs, time, peaks } (time)}
				<PostCard>
					{#snippet author()}
						<PostByline initial="А" name="Аня" {time} color="var(--c)" />
					{/snippet}
					{#snippet media()}
						<VoiceRow
							blobId={`fixture-${durationMs}`}
							{durationMs}
							peaks={displayPeaks(peaks)}
							class={wide ? 'voice-fill-preview' : ''}
						/>
					{/snippet}
					{#snippet reactions()}
						<ReactionBar
							groups={[]}
							keys={[]}
							showAdd
							onopenList={() => {}}
							onadd={() => {}}
							onpick={() => {}}
						/>
					{/snippet}
				</PostCard>
			{/each}
		</div>
	</CircleLayout>
</div>

<style>
	.preview {
		position: fixed;
		inset: 0;
	}
	.preview :global(.voice-fill-preview .vwave) {
		justify-content: space-between;
		gap: min(1.5px, 0.5%);
	}
</style>
