<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	// class — чтобы место вызова сказало, чья это кнопка. Форму аватаров
	// (A16, body[data-avatars]) получает только `.addph.ident-photo`.
	let {
		onclick,
		previewUrl,
		class: extra = ''
	}: { onclick: () => void; previewUrl?: string; class?: string } = $props();
</script>

{#if previewUrl}
	<button
		type="button"
		class="addph preview {extra}"
		style:background-image="url({previewUrl})"
		aria-label="сменить фото"
		{onclick}
	></button>
{:else}
	<button type="button" class="addph {extra}" {onclick}>
		<Icon name="plus" />
	</button>
{/if}
