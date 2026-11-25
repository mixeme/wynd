<script lang="ts">
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import type { Snippet } from 'svelte';

	// Ход работы Мастерской (план 54): установка, обновление, бэкап, переезд.
	// Шаги приходят из Go (internal/workshop, StepState).
	type Step = {
		id: string;
		title: string;
		status: 'pending' | 'running' | 'done' | 'failed';
		note?: string;
		latest?: string;
		latest_note?: string;
		// link — действие в строке шага, например «проверить» удостоверение
		// сборки в «Подписи»; нажимает его onlink, сама StepList ничего не открывает.
		link?: { label: string; url: string };
	};

	let {
		steps,
		meter = false,
		class: className = '',
		style = '',
		onlink,
		extra,
		continues = false
	}: {
		steps: Step[];
		meter?: boolean;
		class?: string;
		style?: string;
		onlink?: (url: string) => void;
		/** Дополнительные строки в том же списке (файлы выпуска у «Подписи»). */
		extra?: Snippet;
		/** Сразу ниже идёт второй список шагов того же дела: стык — обычная строка. */
		continues?: boolean;
	} = $props();

	function mark(step: Step): string {
		if (step.status === 'done') return '✓';
		if (step.status === 'failed') return '!';
		return step.status === 'running' ? '…' : '○';
	}
</script>

{#if meter && steps.length}
	<Meter value={steps.filter((step) => step.status === 'done').length} max={steps.length} />
{/if}
<MarkRows class={className} {style} {continues}>
	{#each steps as step (step.id)}
		<MarkRow
			mark={mark(step)}
			tone={step.status === 'done' ? 'ok' : undefined}
			dim={step.status === 'pending'}
		>
			{step.title}{#if step.note}&nbsp;<span class="ins-dim">— {step.note}</span>{/if}
			{#if step.link}&nbsp;<TextButton
					class="link under"
					onclick={() => step.link && onlink?.(step.link.url)}>{step.link.label}</TextButton
				>{/if}
			{#if step.latest}<div class="ins-step-latest ins-mono">{step.latest}</div>{/if}
			{#if step.latest_note}<div class="ins-dim ins-step-note">{step.latest_note}</div>{/if}
		</MarkRow>
	{/each}
	{@render extra?.()}
</MarkRows>

<style>
	.ins-step-latest {
		color: var(--warn);
		font-weight: 600;
		/* Строка сборки — тем же кеглем и интерлиньяжем, что и название шага:
		   иначе зазор до неё и до давности выходит разным (C71.1). */
		font-size: inherit;
		line-height: inherit;
		overflow-wrap: anywhere;
	}
	.ins-step-note {
		margin-top: 0;
	}
</style>
