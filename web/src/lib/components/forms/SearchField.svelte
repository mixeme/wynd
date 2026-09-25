<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let {
		value = $bindable(''),
		placeholder,
		class: className = '',
		style = ''
	}: {
		value?: string;
		placeholder: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<span class="sfield {className}" {style}>
	<Icon name="search" />
	<input type="search" bind:value {placeholder} />
</span>
