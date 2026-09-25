<script lang="ts">
	import StatusIcon from '$ui/admin/StatusIcon.svelte';
	import type { Snippet } from 'svelte';

	type Status = 'ok' | 'warn' | 'bad';

	let {
		status,
		name,
		title,
		description,
		bad = false,
		actions,
		class: className = '',
		style = ''
	}: {
		status: Status;
		name?: string;
		title?: Snippet;
		description?: string;
		bad?: boolean;
		actions?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="chk {className}" class:bad={bad || status === 'bad'} {style}>
	<StatusIcon {status} />
	<div class="g">
		<div class="n">
			{#if title}
				{@render title()}
			{:else}
				{name}
			{/if}
		</div>
		{#if description}
			<div class="d">{description}</div>
		{/if}
	</div>
	{#if actions}
		{@render actions()}
	{/if}
</div>
