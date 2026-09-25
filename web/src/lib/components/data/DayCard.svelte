<script lang="ts">
	import PhotoPlaceholder from '$ui/data/PhotoPlaceholder.svelte';

	let {
		title,
		subtitle,
		cover,
		coverUrl,
		photoCount,
		onclick,
		class: className = '',
		style = ''
	}: {
		title: string;
		subtitle: string;
		cover?: string;
		coverUrl?: string;
		photoCount?: number | string;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

{#snippet body()}
	{#if coverUrl}
		<PhotoPlaceholder {photoCount} compactCount style="aspect-ratio:1/1">
			<img src={coverUrl} alt="" style="width:100%;height:100%;object-fit:cover;border-radius:inherit" />
		</PhotoPlaceholder>
	{:else if cover}
		<PhotoPlaceholder variant={cover} {photoCount} compactCount style="aspect-ratio:1/1" />
	{:else}
		<PhotoPlaceholder empty style="aspect-ratio:1/1" />
	{/if}
	<div style="font-weight:600;margin-top:6px">{title}</div>
	<div style="font-size:11.5px;color:var(--faint)">{subtitle}</div>
{/snippet}

{#if onclick}
	<button type="button" class={className} {style} {onclick}>
		{@render body()}
	</button>
{:else}
	<div class={className} {style}>
		{@render body()}
	</div>
{/if}

<style>
	button {
		display: block;
		width: 100%;
		margin: 0;
		padding: 0;
		border: 0;
		background: none;
		font: inherit;
		color: inherit;
		text-align: left;
		cursor: pointer;
	}
</style>
