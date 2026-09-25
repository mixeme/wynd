<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let {
		label,
		class: className = '',
		style = ''
	}: {
		label: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<span
	class="tm {className}"
	{style}
	style:margin-left="auto"
	style:display="flex"
	style:align-items="center"
	style:gap="5px"
>
	<Icon name="clock" size="xs" />
	{label}
</span>
