<script lang="ts">
	import type { Snippet } from 'svelte';

	// Файл во вкладке «Архивы» Мастерской (план 54, кадры 11б–11г): имя,
	// под ним пояснение или ошибка, справа действие («другой», «✕»).
	let {
		name,
		detail,
		error,
		action,
		class: className = '',
		style = ''
	}: {
		name: string;
		detail?: string;
		error?: string;
		action?: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="ins-file {className}" {style}>
	<span class="ins-mark">·</span>
	<div>
		{name}
		{#if error}
			<div class="ins-err">{error}</div>
		{:else if detail}
			<div class="ins-dim">{detail}</div>
		{/if}
	</div>
	{@render action?.()}
</div>

<style>
	.ins-file {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		padding: 8px 0;
		border-bottom: 1px solid var(--line);
	}
	.ins-file > div {
		flex: 1;
		min-width: 0;
	}
	.ins-file :global(.ins-x) {
		flex: none;
	}
	.ins-err {
		color: var(--bad);
	}
</style>
