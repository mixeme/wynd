<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let {
		label,
		count,
		expanded = true,
		onclick,
		class: className = '',
		style = ''
	}: {
		label: string;
		count?: number | string;
		expanded?: boolean;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

{#if onclick}
	<button type="button" class="fold {className}" {style} {onclick}>
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
