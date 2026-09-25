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
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	class="pic {cover ?? ''} {className}"
	class:cover-click={Boolean(oncover)}
	{style}
	style:aspect-ratio="16/9"
	style:border-radius="0"
	style:border="0"
	style:border-bottom="1px solid var(--line)"
	onclick={oncover}
>
	{#if coverUrl}
		<img src={coverUrl} alt="" />
	{/if}
	<span class="tagr">обложка дня</span>
	<span class="cnt">сменить</span>
</div>
{#if title && subtitle}
	{#if ontitle}
		<SettingsRow {title} {subtitle} chevron={false} onclick={ontitle} />
	{:else}
		<SettingsRow {title} {subtitle} chevron={false} />
	{/if}
{/if}

<style>
	.cover-click {
		cursor: pointer;
	}
	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
</style>
