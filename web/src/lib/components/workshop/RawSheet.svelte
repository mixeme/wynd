<script lang="ts">
	import Button from '$ui/forms/Button.svelte';

	// «Что проверили» — окно поверх экрана осмотра (план 54, кадр 2е): читать
	// ответы сервера, а не подглядывать в три строки. Раздел, из-за которого
	// отказ, — первым и выделен. Escape закрывает.
	type Part = { name: string; body: string; cause: boolean };

	let {
		parts,
		copied = false,
		oncopy,
		onclose
	}: {
		parts: Part[];
		/** Только что скопировали — на кнопке «Скопировано». */
		copied?: boolean;
		oncopy: () => void;
		onclose: () => void;
	} = $props();
</script>

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape') onclose();
	}}
/>

<div class="ins-over" role="presentation" onclick={onclose}></div>
<div class="ins-sheet" role="dialog" aria-modal="true" aria-label="Что проверили">
	<div class="ins-sheet-head">
		<b>Что проверили</b>
		<span class="ins-dim">ответы сервера на команды осмотра</span>
		<button class="ins-x" type="button" aria-label="Закрыть" onclick={onclose}>✕</button>
	</div>
	<div class="ins-raw">
		{#each parts as s, i (i)}
			<div class:ins-cause-part={s.cause}>
				{#if s.cause}<b class="ins-cause">## {s.name} — из-за этого отказ</b>{:else}## {s.name}{/if}
				{#if s.body}<div>{s.body}</div>{/if}
			</div>
		{/each}
	</div>
	<div class="ins-sheet-foot">
		<Button variant="ghost" onclick={oncopy}>{copied ? 'Скопировано' : 'Скопировать всё'}</Button>
		<Button variant="ghost" onclick={onclose}>Закрыть</Button>
	</div>
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
