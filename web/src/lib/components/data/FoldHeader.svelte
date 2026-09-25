<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let {
		label,
		count,
		expanded = true,
		class: className = '',
		style = ''
	}: {
		label: string;
		count?: number | string;
		expanded?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="fold {className}" {style}>
	<Icon name={expanded ? 'chev' : 'chevr'} size="sm" />
	<span>{label}</span>
	{#if count !== undefined}
		<span class="cnt2">{count}</span>
	{/if}
</div>
