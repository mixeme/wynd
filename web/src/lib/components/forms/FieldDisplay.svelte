<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		value,
		active = false,
		gray = false,
		mono = false,
		admin = false,
		class: className = '',
		style = '',
		children
	}: {
		value?: string;
		active?: boolean;
		gray?: boolean;
		mono?: boolean;
		admin?: boolean;
		class?: string;
		style?: string;
		children?: Snippet;
	} = $props();
</script>

<div
	class="{admin ? 'inp' : 'fld'} {className}"
	class:act={active}
	class:gr={gray}
	class:mono={mono}
	{style}
>
	{#if children}
		{@render children()}
	{:else}
		{value}
	{/if}
</div>
