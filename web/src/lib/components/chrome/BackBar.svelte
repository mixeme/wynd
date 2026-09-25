<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import type { Snippet } from 'svelte';

	let {
		title,
		right,
		search,
		compact = false,
		onback,
		children
	}: {
		title?: string;
		right?: string;
		search?: string;
		compact?: boolean;
		onback?: () => void;
		children?: Snippet;
	} = $props();
</script>

<div class="bar" style:padding-bottom={compact ? '10px' : undefined}>
	{#if onback}
		<IconButton name="back" label="Назад" onclick={() => onback()} />
	{/if}
	{#if search !== undefined}
		<span class="sfield">
			<Icon name="search" />
			<span>{search}</span>
		</span>
	{:else if children}
		{@render children()}
	{:else}
		<span class="t">{title}</span>
		{#if right}
			<span class="rt">{right}</span>
		{/if}
	{/if}
</div>
