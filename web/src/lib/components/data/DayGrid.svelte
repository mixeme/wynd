<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div
	class="day-grid {className}"
	{style}
	style:display="grid"
	style:grid-template-columns="1fr 1fr"
	style:gap="12px 10px"
	style:padding="0 14px"
>
	{@render children()}
</div>
