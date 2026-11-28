<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import { onMount, type Snippet } from 'svelte';

	// «Что проверили» — окно поверх экрана осмотра (план 54, кадр 2е): читать
	// ответы сервера, а не подглядывать в три строки. Раздел, из-за которого
	// отказ, — первым и выделен. Escape закрывает.
	type Part = { name: string; body: string; cause: boolean };

	let {
		parts = [],
		copied = false,
		oncopy,
		onclose,
		title = 'Что проверили',
		style = '',
		dismissible = true,
		children
	}: {
		parts?: Part[];
		/** Только что скопировали — на кнопке «Скопировано». */
		copied?: boolean;
		oncopy?: () => void;
		onclose: () => void;
		title?: string;
		style?: string;
		dismissible?: boolean;
		children?: Snippet;
	} = $props();
	let sheet: HTMLDivElement;
	function dismiss() {
		if (dismissible) onclose();
	}

	onMount(() => {
		if (children) sheet.focus({ preventScroll: true });
	});

	function keydown(event: KeyboardEvent) {
		if (event.key === 'Escape') dismiss();
		if (!children || event.key !== 'Tab') return;
		const controls = [...sheet.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)')];
		const edge = event.shiftKey ? controls[0] : controls.at(-1);
		if (document.activeElement === edge || !controls.includes(document.activeElement as HTMLElement)) {
			event.preventDefault();
			(event.shiftKey ? controls.at(-1) : controls[0])?.focus();
		}
	}
</script>

<svelte:window onkeydown={keydown} />

<div class="ins-over" role="presentation" onclick={dismiss}></div>
<div class="ins-sheet" role="dialog" aria-modal="true" aria-label={title} {style} tabindex={children ? -1 : undefined} bind:this={sheet}>
	<div class="ins-sheet-head">
		<b>{title}</b>
		{#if !children}<span class="ins-dim">ответы сервера на команды осмотра</span>{/if}
		<button class="ins-x" type="button" aria-label="Закрыть" disabled={!dismissible} onclick={dismiss}>✕</button>
	</div>
	{#if children}
		{@render children()}
	{:else}
		<div class="ins-raw">
			{#each parts as s, i (i)}
				<div class:ins-cause-part={s.cause}>
					{#if s.cause}<b class="ins-cause">## {s.name} — из-за этого отказ</b>{:else}## {s.name}{/if}
					{#if s.body}<div>{s.body}</div>{/if}
				</div>
			{/each}
		</div>
		<div class="ins-sheet-foot">
			{#if oncopy}<Button variant="ghost" onclick={oncopy}>{copied ? 'Скопировано' : 'Скопировать всё'}</Button>{/if}
			<Button variant="ghost" onclick={dismiss}>Закрыть</Button>
		</div>
	{/if}
</div>

<style>
	.ins-over {
		position: absolute;
		inset: 0;
		background: rgba(43, 39, 36, 0.38);
	}
	.ins-sheet {
		position: absolute;
		inset: 16px 18px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 14px 16px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--paper);
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.25);
	}
	.ins-sheet-head {
		display: flex;
		align-items: baseline;
		gap: 10px;
	}
	.ins-sheet-head b {
		font-size: 15px;
	}
	.ins-sheet-foot {
		display: flex;
		justify-content: space-between;
		gap: 10px;
	}
	.ins-cause {
		color: var(--bad);
	}
	.ins-cause-part {
		margin-bottom: 1.45em;
	}
</style>
