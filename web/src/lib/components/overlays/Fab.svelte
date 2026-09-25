<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		dark = false,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		dark?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div
	class="fab {className}"
	style:background={dark ? '#EFE9E0' : undefined}
	style:color={dark ? '#211E1C' : undefined}
	{style}
>
	{@render children()}
</div>
