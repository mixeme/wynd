<script lang="ts">
	import FoldHeader from '$ui/data/FoldHeader.svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	let {
		label,
		count,
		expanded = true,
		onclick,
		actionLabel,
		onaction,
		actionLabel2,
		onaction2,
		class: className = '',
		style = '',
		foldStyle = '',
		...foldRest
	}: {
		label: string;
		count?: number | string;
		expanded?: boolean;
		onclick?: () => void;
		actionLabel?: string;
		onaction?: () => void;
		actionLabel2?: string;
		onaction2?: () => void;
		class?: string;
		style?: string;
		foldStyle?: string;
	} & HTMLButtonAttributes = $props();

	const showAction = $derived(Boolean(actionLabel && onaction));
	const showAction2 = $derived(Boolean(actionLabel2 && onaction2));
</script>

<div class="circle-row-card {className}" {style}>
	<FoldHeader
		{label}
		{count}
		{expanded}
		{onclick}
		style={foldStyle}
		{...foldRest}
	/>
	{#if showAction}
		<button type="button" class="circle-row-action" onclick={() => onaction?.()}>
			{actionLabel}
		</button>
	{/if}
	{#if showAction2}
		<button type="button" class="circle-row-action" onclick={() => onaction2?.()}>
			{actionLabel2}
		</button>
	{/if}
</div>
