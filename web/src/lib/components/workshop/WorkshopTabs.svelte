<script lang="ts" generics="T extends string">
	// Вкладки окна Мастерской под полосой заголовка (план 54; кадры 1, 11а):
	// «Серверы» и «Архивы».
	let {
		tabs,
		value = $bindable(),
		class: className = '',
		style = ''
	}: {
		tabs: { id: T; label: string }[];
		value: T;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="ins-tabs {className}" role="tablist" {style}>
	{#each tabs as tab (tab.id)}
		<button
			type="button"
			role="tab"
			class:on={value === tab.id}
			aria-selected={value === tab.id}
			onclick={() => (value = tab.id)}
		>
			{tab.label}
		</button>
	{/each}
</div>

<style>
	.ins-tabs {
		display: flex;
		gap: 22px;
		padding: 0 24px;
		border-bottom: 1px solid var(--line);
		background: var(--paper);
	}
	.ins-tabs button {
		margin-bottom: -1px;
		padding: 10px 0 9px;
		border: 0;
		border-bottom: 2px solid transparent;
		background: none;
		color: var(--muted);
		font: inherit;
		cursor: pointer;
	}
	.ins-tabs button.on {
		border-bottom-color: var(--terracotta);
		color: var(--ink);
		font-weight: 600;
	}
</style>
