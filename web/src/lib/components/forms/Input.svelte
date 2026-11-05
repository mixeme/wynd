<script lang="ts">
	import type { HTMLInputAttributes, HTMLSelectAttributes } from 'svelte/elements';

	let {
		value = $bindable<string | number>(''),
		admin = false,
		active = false,
		gray = false,
		mono = false,
		small = false,
		class: className = '',
		style = '',
		type = 'text',
		options,
		...rest
	}: {
		value?: string | number;
		admin?: boolean;
		active?: boolean;
		gray?: boolean;
		mono?: boolean;
		/** Мелкий шрифт 12,5 px — длинное значение в узком поле (адрес сервера). */
		small?: boolean;
		class?: string;
		style?: string;
		type?: HTMLInputElement['type'];
		options?: { value: string; label: string }[];
	} & HTMLInputAttributes = $props();
</script>

{#if options}
	<select
		class="{admin ? 'inp' : 'fld'} {className}"
		{style}
		bind:value
		{...rest as unknown as HTMLSelectAttributes}
	>
		{#each options as option (option.value)}<option value={option.value}>{option.label}</option
			>{/each}
	</select>
{:else}
	<input
		class="{admin ? 'inp' : 'fld'} {className}"
		class:act={active}
		class:gr={gray}
		class:mono
		class:sm={small}
		{type}
		{style}
		bind:value
		{...rest}
	/>
{/if}
