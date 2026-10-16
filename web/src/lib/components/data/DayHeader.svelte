<script lang="ts">
	import TextButton from '$ui/forms/TextButton.svelte';

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
<!-- Название — заголовком под обложкой, дата — в шапке (C44, слово владельца):
     длинное название в шапку не влезало. Без названия заголовка нет; кто может
     назвать день, видит тихую строку «назвать день». -->
{#if title && ontitle}
	<button type="button" class="dayh" onclick={ontitle}>
		<span class="dayh-name">{title}</span>
		{#if subtitle}<span class="dayh-sub">{subtitle}</span>{/if}
	</button>
{:else if title}
	<div class="dayh"><span class="dayh-name">{title}</span></div>
{:else if ontitle}
	<div class="dayh"><TextButton onclick={ontitle}>назвать день</TextButton></div>
{/if}

<style>
	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.dayh {
		display: block;
		width: 100%;
		padding: 14px 16px 2px;
		border: 0;
		background: none;
		color: inherit;
		font: inherit;
		text-align: left;
	}
	.dayh-name {
		display: block;
		font-size: 22px;
		font-weight: 600;
		line-height: 1.25;
		overflow-wrap: anywhere;
	}
	.dayh-sub {
		display: block;
		margin-top: 3px;
		font-size: 12.5px;
		color: var(--muted);
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
