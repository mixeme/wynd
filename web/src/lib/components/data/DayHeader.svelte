<script lang="ts">
	import SettingsRow from '$ui/data/SettingsRow.svelte';

	let {
		cover,
		coverUrl,
		title,
		subtitle,
		oncover,
		ontitle,
		class: className = '',
		style = ''
	}: {
		cover?: string;
		coverUrl?: string;
		title?: string;
		subtitle?: string;
		oncover?: () => void;
		ontitle?: () => void;
		class?: string;
		style?: string;
	} = $props();

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
			<img src={coverUrl} alt="" />
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
			<img src={coverUrl} alt="" />
		{/if}
		<span class="tagr">обложка дня</span>
		<span class="cnt">сменить</span>
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
	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
</style>
