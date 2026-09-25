<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	let {
		label,
		count,
		expanded = true,
		onclick,
		class: className = '',
		style = '',
		...rest
	}: {
		label: string;
		count?: number | string;
		expanded?: boolean;
		onclick?: () => void;
		class?: string;
		style?: string;
	} & HTMLButtonAttributes = $props();
</script>

{#if onclick}
	<button type="button" class="fold {className}" {style} {onclick} {...rest}>
		<Icon name={expanded ? 'chev' : 'chevr'} size="sm" />
		<span>{label}</span>
		{#if count !== undefined}
			<span class="cnt2">{count}</span>
		{/if}
	</button>
{:else}
	<div class="fold {className}" {style}>
		<Icon name={expanded ? 'chev' : 'chevr'} size="sm" />
		<span>{label}</span>
		{#if count !== undefined}
			<span class="cnt2">{count}</span>
		{/if}
	</div>
{/if}
