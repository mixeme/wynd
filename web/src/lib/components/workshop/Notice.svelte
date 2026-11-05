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
		profile,
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
		profile?: {
			name: string;
			host: string;
			user: string;
			domain?: string;
			status: string;
			state: string;
		};
		class?: string;
		style?: string;
	} = $props();
</script>

{#if profile}
	<header class="ins-server-context">
		<div class="ins-server-heading">
			<h3>{profile.name}</h3>
			<span class="ins-connection-badge" data-state={profile.state}>{profile.status}</span>
		</div>
		<div class="ins-identity">
			SSH: {profile.user}@{profile.host}<br />Сайт: {profile.domain || 'ещё не установлен'}
		</div>
		<div class="ins-connection-row">{@render children?.()}</div>
	</header>
{:else}
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
{/if}

<style>
	.ins-server-context {
		border-bottom: 1px solid var(--line);
		padding-bottom: 16px;
		margin-bottom: 18px;
	}
	.ins-server-heading {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
		margin-bottom: 4px;
	}
	.ins-server-heading h3 {
		margin: 0;
		font-size: 15px;
		font-weight: 500;
	}
	.ins-connection-badge {
		display: inline-flex;
		align-items: center;
		border-radius: 999px;
		padding: 2px 9px;
		font-size: 12px;
		font-weight: 400;
		color: var(--muted);
		background: light-dark(#e9e3da, #3d352c);
		white-space: normal;
	}
	.ins-connection-badge[data-state='connected'] {
		color: var(--ok);
		background: light-dark(#e5ebdb, #333d2a);
	}
	.ins-connection-badge[data-state='error'] {
		color: var(--bad);
		background: light-dark(#f2e2dc, #49332d);
	}
	.ins-identity {
		font-size: 12px;
		color: var(--muted);
		overflow-wrap: anywhere;
	}
	.ins-connection-row {
		display: flex;
		gap: 13px;
		align-items: center;
		flex-wrap: wrap;
		margin-top: 12px;
	}
	.ins-connection-row :global(.ins-check) {
		margin: 0;
	}
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
