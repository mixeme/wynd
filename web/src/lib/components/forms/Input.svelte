<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	let {
		value = $bindable<string | number>(''),
		admin = false,
		active = false,
		gray = false,
		mono = false,
		class: className = '',
		style = '',
		type = 'text',
		...rest
	}: {
		value?: string | number;
		admin?: boolean;
		active?: boolean;
		gray?: boolean;
		mono?: boolean;
		class?: string;
		style?: string;
		type?: HTMLInputElement['type'];
	} & HTMLInputAttributes = $props();
</script>

<input
	class="{admin ? 'inp' : 'fld'} {className}"
	class:act={active}
	class:gr={gray}
	class:mono={mono}
	{type}
	{style}
	bind:value
	{...rest}
/>
