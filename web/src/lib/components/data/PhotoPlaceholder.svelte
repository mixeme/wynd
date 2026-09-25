<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		variant,
		photoCount,
		empty = false,
		compactCount = false,
		children,
		class: className = '',
		style = ''
	}: {
		variant?: string;
		photoCount?: number | string;
		empty?: boolean;
		compactCount?: boolean;
		children?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

{#if empty}
	<div
		class="ph-empty {className}"
		{style}
		style:aspect-ratio="1/1"
		style:border="1px dashed var(--line)"
		style:border-radius="10px"
		style:display="grid"
		style:place-items="center"
		style:color="var(--faint)"
		style:font-size="11.5px"
	>
		без фотографий
	</div>
{:else}
	<div class="pic {variant ?? ''} {className}" {style}>
		{@render children?.()}
		{#if photoCount !== undefined}
			<span
				class="cnt"
				style={compactCount ? 'bottom:6px;right:6px;padding:3px 8px' : undefined}
				>{photoCount}</span
			>
		{/if}
	</div>
{/if}
