<script lang="ts">
	import SettingsRow from '$ui/data/SettingsRow.svelte';

	let {
		cover,
		coverUrl,
		empty = false,
		title,
		subtitle,
		oncover,
		ontitle,
		class: className = '',
		style = ''
	}: {
		cover?: string;
		coverUrl?: string;
		/**
		 * В дне нет ни одного снимка: вместо обложки — рамка «без фотографий»,
		 * как у карточки в списке дней (5.1). Полосатая заглушка читалась как
		 * картинка, а не как её отсутствие.
		 */
		empty?: boolean;
		title?: string;
		subtitle?: string;
		oncover?: () => void;
		ontitle?: () => void;
		class?: string;
		style?: string;
	} = $props();

	// Снимок есть, но ещё едет — ровный фон, а не полосы: полосы значат макет.
	const picClass = $derived(
		[cover, !cover && !coverUrl ? 'wait' : '', className].filter(Boolean).join(' ')
	);
</script>

{#if empty}
	<div class="none {className}" {style}>без фотографий</div>
{:else if oncover}
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
	.wait {
		background: var(--tint);
	}
	.none {
		display: grid;
		place-items: center;
		aspect-ratio: 3/1;
		margin: 12px 14px 0;
		border: 1px dashed var(--line);
		border-radius: 10px;
		color: var(--faint);
		font-size: 11.5px;
	}
</style>
