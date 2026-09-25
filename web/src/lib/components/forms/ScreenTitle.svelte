<script lang="ts">
	let {
		children,
		centered = false,
		class: className = '',
		style = ''
	}: {
		children: import('svelte').Snippet;
		centered?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="h1s {className}" class:ctr={centered} {style}>
	{@render children()}
</div>
