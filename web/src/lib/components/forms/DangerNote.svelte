<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		children,
		label = 'Необратимо',
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		label?: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="danger {className}" {style}>
	<div class="dl">{label}</div>
	<div class="note">
		{@render children()}
	</div>
</div>

<style>
	.note {
		padding: 0 14px 13px;
		font-size: 12.5px;
		color: var(--muted);
		line-height: 1.5;
	}
</style>
