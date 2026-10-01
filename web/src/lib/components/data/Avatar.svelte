<script lang="ts">
	let {
		initial,
		color,
		src,
		size = 'md',
		class: className = '',
		style = ''
	}: {
		initial: string;
		color?: string;
		src?: string;
		/** lg — крупный, по центру: «Кто вы в этом круге» (план 47, 1.9). */
		size?: 'md' | 'lg';
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="av {size === 'lg' ? 'av-lg ' : ''}{className}" style:background={src ? undefined : color} {style}>
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
