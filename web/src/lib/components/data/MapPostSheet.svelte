<script lang="ts">
	import BylineParts from '$ui/data/BylineParts.svelte';

	let {
		thumbUrl,
		author,
		time,
		day,
		body,
		onclick,
		class: className = '',
		style = ''
	}: {
		thumbUrl?: string;
		author: string;
		time: string;
		/** День записи, если он не день публикации (C49). */
		day?: string;
		body: string;
		onclick: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<button type="button" class="map-sheet {className}" {style} {onclick}>
	<div class="thumb">
		{#if thumbUrl}
			<img src={thumbUrl} alt="" />
		{/if}
	</div>
	<div class="map-sheet-body">
		<div class="map-sheet-author">{author}</div>
		<div class="map-sheet-time">
			<BylineParts parts={day ? [{ text: time }, { text: day, icon: 'day' }] : [{ text: time }]} />
		</div>
		<div class="map-sheet-text">{body}</div>
	</div>
</button>
