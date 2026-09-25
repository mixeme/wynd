<script lang="ts">
	import type { HTMLTextareaAttributes } from 'svelte/elements';

	let {
		value = $bindable(''),
		variant = 'area',
		active = false,
		class: className = '',
		style = '',
		...rest
	}: {
		value?: string;
		variant?: 'area' | 'field';
		active?: boolean;
		class?: string;
		style?: string;
	} & HTMLTextareaAttributes = $props();

	const rootClass = $derived(variant === 'field' ? 'fld' : 'ta');
</script>

<textarea class="{rootClass} {className}" class:act={active} {style} bind:value {...rest}></textarea>
