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

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class={className} {style} {onclick}>
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
</div>
