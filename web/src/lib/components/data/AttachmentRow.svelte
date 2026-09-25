<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let {
		filename,
		size,
		class: className = '',
		style = '',
		onclick
	}: {
		filename: string;
		size: string;
		class?: string;
		style?: string;
		onclick?: () => void;
	} = $props();

	function onClick(e: MouseEvent) {
		e.stopPropagation();
		onclick?.();
	}
</script>

{#if onclick}
	<button type="button" class="att {className}" {style} onclick={onClick}>
		<Icon name="file" />
		<div class="g" style:flex="1">
			<div style="font-size:12.5px">{filename}</div>
			<div class="sz">{size}</div>
		</div>
	</button>
{:else}
	<div class="att {className}" {style}>
		<Icon name="file" />
		<div class="g" style:flex="1">
			<div style="font-size:12.5px">{filename}</div>
			<div class="sz">{size}</div>
		</div>
	</div>
{/if}
