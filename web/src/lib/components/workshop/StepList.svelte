<script lang="ts">
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';

	// Ход работы Мастерской (план 54): установка, обновление, бэкап, переезд.
	// Шаги приходят из Go (internal/installer, StepState).
	type Step = {
		id: string;
		title: string;
		status: 'pending' | 'running' | 'done' | 'failed';
		note?: string;
	};

	let {
		steps,
		class: className = '',
		style = ''
	}: {
		steps: Step[];
		class?: string;
		style?: string;
	} = $props();

	function mark(step: Step): string {
		if (step.status === 'done') return '✓';
		if (step.status === 'failed') return '!';
		return step.status === 'running' ? '…' : '○';
	}
</script>

<MarkRows class={className} {style}>
	{#each steps as step (step.id)}
		<MarkRow
			mark={mark(step)}
			tone={step.status === 'done' ? 'ok' : undefined}
			dim={step.status === 'pending'}
		>
			{step.title}{#if step.note}&nbsp;<span class="ins-dim">— {step.note}</span>{/if}
		</MarkRow>
	{/each}
</MarkRows>
