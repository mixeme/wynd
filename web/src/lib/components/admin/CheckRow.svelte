<script lang="ts">
	import StatusIcon from '$ui/admin/StatusIcon.svelte';
	import type { Snippet } from 'svelte';

	type Status = 'ok' | 'warn' | 'bad';

	let {
		status,
		dotColor,
		name,
		title,
		description,
		bad = false,
		actions,
		class: className = '',
		style = ''
	}: {
		status?: Status;
		/** Цветная точка вместо значка статуса — круг в карточке человека (план 47, 1.5). */
		dotColor?: string;
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
	{#if dotColor}
		<span class="dot chk-dot" style:background={dotColor}></span>
	{:else if status}
		<StatusIcon {status} />
	{/if}
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
