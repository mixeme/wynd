<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		label = 'Необратимо',
		title,
		action,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		label?: string;
		/** Что именно необратимо: «Удалить с сервера» (план 47, 1.6). */
		title?: string;
		/** Кнопка под пояснением. */
		action?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

{#if title}
	<div class="danger titled {className}" {style}>
		<div class="dl">{label}</div>
		<div class="dt">{title}</div>
		<div class="dd note lh-15">{@render children()}</div>
		{#if action}
			<div class="dd">{@render action()}</div>
		{/if}
	</div>
{:else}
	<div class="danger {className}" {style}>
		<div class="dl">{label}</div>
		<div class="note">
			{@render children()}
		</div>
	</div>
{/if}

<style>
	.note {
		padding: 0 14px 13px;
		font-size: 12.5px;
		color: var(--muted);
		line-height: 1.5;
	}
</style>
