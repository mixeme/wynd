<script lang="ts">
	let {
		initial,
		color,
		src,
		class: className = '',
		style = ''
	}: {
		initial: string;
		color?: string;
		src?: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="av {className}" style:background={src ? undefined : color} {style}>
	{#if src}
		<img {src} alt="" />
	{:else}
		{initial}
	{/if}
</div>

<style>
	.av img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: inherit;
		display: block;
	}
</style>
