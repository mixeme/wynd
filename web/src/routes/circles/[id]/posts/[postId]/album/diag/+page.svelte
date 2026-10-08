<script lang="ts">
	// Временная страница (план 46, C26): диагностика видео. Убрать вместе с
	// $ui/overlays/VideoDiag.svelte, когда причина найдена.
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import VideoDiag from '$ui/overlays/VideoDiag.svelte';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
</script>

<VideoDiag
	origin={circle.origin}
	blobId={$page.url.searchParams.get('m') ?? ''}
	posterId={$page.url.searchParams.get('p') ?? ''}
/>
