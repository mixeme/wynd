<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import { onMount } from 'svelte';

	let {
		value = $bindable(''),
		placeholder,
		autofocus = false,
		class: className = '',
		style = ''
	}: {
		value?: string;
		placeholder: string;
		// Экран поиска открывают, чтобы печатать: без фокуса на телефоне
		// приходилось ещё раз попадать в поле.
		autofocus?: boolean;
		class?: string;
		style?: string;
	} = $props();

	let input: HTMLInputElement | undefined = $state();

	onMount(() => {
		if (autofocus) input?.focus();
	});
</script>

<span class="sfield {className}" {style}>
	<Icon name="search" />
	<input type="search" bind:this={input} bind:value {placeholder} />
</span>
