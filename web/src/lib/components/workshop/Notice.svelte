<script lang="ts">
	import type { Snippet } from 'svelte';

	// Плашка окна Мастерской (план 54): итог, отказ, пояснение к шагу.
	// title — жирная строка, text — пояснение под ней; остальное — children.
	// action — кнопка справа от текста (кадр 7а, «Вышла версия»).
	let {
		tone,
		mono = false,
		alert = false,
		title,
		text,
		action,
		children,
		class: className = '',
		style = ''
	}: {
		/** bad — отказ, рамка красноватая; note — итог над заголовком экрана, приглушённый. */
		tone?: 'bad' | 'note';
		mono?: boolean;
		alert?: boolean;
		title?: string;
		text?: string;
		action?: Snippet;
		children?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div
	class="ins-box {className}"
	class:bad={tone === 'bad'}
	class:ins-note={tone === 'note'}
	class:ins-mono={mono}
	class:ins-update={Boolean(action)}
	role={alert ? 'alert' : undefined}
	{style}
>
	{#if action}
		<div>
			{#if title}<div class="ins-strong">{title}</div>{/if}
			{#if text}<div class="ins-dim">{text}</div>{/if}
			{@render children?.()}
		</div>
		{@render action()}
	{:else}
		{#if title}<div class="ins-strong">{title}</div>{/if}
		{#if text}<div class="ins-dim">{text}</div>{/if}
		{@render children?.()}
	{/if}
</div>

<style>
	.ins-box {
		margin-top: 12px;
		padding: 12px 14px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--card);
		line-height: 1.5;
	}
	/* Итог удаления над экраном помехи: рамка стоит выше заголовка. */
	.ins-box.ins-note {
		margin: 0 0 16px;
	}
	.ins-box.bad {
		border-color: color-mix(in srgb, var(--bad) 45%, var(--line));
	}
	.ins-box :global(.ins-strong + *) {
		margin-top: 6px;
	}
	.ins-box :global(.ins-dim + .ins-dim) {
		margin-top: 8px;
	}
	.ins-box :global(.ins-mono) {
		word-break: break-all;
	}
	.ins-box.ins-update {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 10px;
	}
	.ins-box.ins-update > div {
		flex: 1;
	}
</style>
