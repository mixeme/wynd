<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import type { Snippet } from 'svelte';

	let {
		title,
		children,
		primaryLabel,
		secondaryLabel,
		onprimary,
		onsecondary,
		class: className = '',
		style = ''
	}: {
		title: string;
		children: Snippet;
		primaryLabel: string;
		secondaryLabel: string;
		onprimary: () => void;
		onsecondary: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="post day-prompt {className}" {style}>
	<div class="day-prompt-title">{title}</div>
	<div class="day-prompt-body">
		{@render children()}
	</div>
	<div class="rowin">
		<Button variant="colored" style="flex:1;margin:0" onclick={onprimary}>{primaryLabel}</Button>
		<Button variant="ghost" style="flex:1;margin:0" onclick={onsecondary}>{secondaryLabel}</Button>
	</div>
</div>
