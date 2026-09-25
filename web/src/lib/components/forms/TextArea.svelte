<script lang="ts">
	import type { HTMLTextareaAttributes } from 'svelte/elements';

	let {
		value = $bindable(''),
		variant = 'area',
		active = false,
		class: className = '',
		style = '',
		el = $bindable<HTMLTextAreaElement>(),
		...rest
	}: {
		value?: string;
		variant?: 'area' | 'field' | 'compose' | 'comment';
		active?: boolean;
		class?: string;
		style?: string;
		el?: HTMLTextAreaElement;
	} & HTMLTextareaAttributes = $props();

	const rootClass = $derived(
		variant === 'field'
			? 'fld'
			: variant === 'compose'
				? 'compose-text'
				: variant === 'comment'
					? 'inp'
					: 'ta'
	);
</script>

<textarea
	class="{rootClass} {className}"
	class:act={active}
	{style}
	bind:this={el}
	bind:value
	{...rest}
></textarea>
