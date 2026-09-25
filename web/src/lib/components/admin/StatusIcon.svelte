<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	type Status = 'ok' | 'warn' | 'bad';

	let {
		status,
		class: className = '',
		style = ''
	}: {
		status: Status;
		class?: string;
		style?: string;
	} = $props();
</script>

<span class="st {status} {className}" {style}>
	{#if status === 'ok'}
		<Icon name="check" />
	{:else if status === 'bad'}
		<Icon name="x" />
	{:else}
		!
	{/if}
</span>
