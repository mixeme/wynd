<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		album = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		/** Сетка альбома (4.13, 5.3): поля 12 px по бокам и 16 снизу (план 47, 4). */
		album?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="g3 {album ? 'album ' : ''}{className}" {style}>
	{@render children()}
</div>
