<script lang="ts">
	let {
		thumbUrl,
		author,
		time,
		body,
		onclick,
		class: className = '',
		style = ''
	}: {
		thumbUrl?: string;
		author: string;
		time: string;
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
		<div class="map-sheet-time">{time}</div>
		<div class="map-sheet-text">{body}</div>
	</div>
</button>
