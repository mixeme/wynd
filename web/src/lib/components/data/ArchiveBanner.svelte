<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '$ui/Icon.svelte';

	let {
		title,
		children,
		action = 'Скачать архив',
		onaction,
		class: className = '',
		style = ''
	}: {
		title: string;
		children: Snippet;
		action?: string;
		onaction?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="danger {className}" {style} style:margin="12px 14px 0">
	<div class="dl">{title}</div>
	<div class="note">
		{@render children()}
	</div>
	{#if onaction}
		<button
			type="button"
			class="di"
			style="border-top:1px solid var(--line);display:flex;align-items:center"
			onclick={() => onaction()}
		>
			{action}
			<Icon name="chevr" size="sm" style="margin-left:auto" />
		</button>
	{/if}
</div>

<style>
	.note {
		padding: 0 14px 12px;
		font-size: 12.5px;
		color: var(--muted);
		line-height: 1.5;
	}
</style>
