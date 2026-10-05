<script lang="ts">
	import SettingsRow from '$ui/data/SettingsRow.svelte';

	let {
		cover,
		coverUrl,
		kind = 'photo',
		title,
		subtitle,
		oncover,
		ontitle,
		class: className = '',
		style = ''
	}: {
		cover?: string;
		coverUrl?: string;
		/** video — файл ролика без JPEG кадра. */
		kind?: 'photo' | 'video';
		title?: string;
		subtitle?: string;
		oncover?: () => void;
		ontitle?: () => void;
		class?: string;
		style?: string;
	} = $props();

	function showFirstFrame(e: Event) {
		const video = e.currentTarget as HTMLVideoElement;
		if (video.currentTime === 0) video.currentTime = 0.001;
	}

	const picClass = $derived([cover, className].filter(Boolean).join(' '));
</script>

{#if oncover}
	<button
		type="button"
		class="pic {picClass}"
		{style}
		style:aspect-ratio="16/9"
		style:border-radius="0"
		style:border="0"
		style:border-bottom="1px solid var(--line)"
		aria-label="Сменить обложку"
		onclick={oncover}
	>
		{#if coverUrl}
			{#if kind === 'video'}
				<video src={coverUrl} muted playsinline preload="metadata" onloadedmetadata={showFirstFrame}></video>
			{:else}
				<img src={coverUrl} alt="" />
			{/if}
		{/if}
		<span class="tagr">обложка дня</span>
		<span class="cnt">сменить</span>
	</button>
{:else}
	<div
		class="pic {picClass}"
		{style}
		style:aspect-ratio="16/9"
		style:border-radius="0"
		style:border="0"
		style:border-bottom="1px solid var(--line)"
	>
		{#if coverUrl}
			{#if kind === 'video'}
				<video src={coverUrl} muted playsinline preload="metadata" onloadedmetadata={showFirstFrame}></video>
			{:else}
				<img src={coverUrl} alt="" />
			{/if}
		{/if}
		<span class="tagr">обложка дня</span>
	</div>
{/if}
{#if title && subtitle}
	{#if ontitle}
		<SettingsRow {title} {subtitle} chevron={false} onclick={ontitle} />
	{:else}
		<SettingsRow {title} {subtitle} chevron={false} />
	{/if}
{/if}

<style>
	img,
	video {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
</style>
