<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let { onclick, previewUrl }: { onclick: () => void; previewUrl?: string } = $props();
</script>

{#if previewUrl}
	<button
		type="button"
		class="addph preview"
		style:background-image="url({previewUrl})"
		aria-label="сменить фото"
		{onclick}
	></button>
{:else}
	<button type="button" class="addph" {onclick}>
		<Icon name="plus" />
	</button>
{/if}
