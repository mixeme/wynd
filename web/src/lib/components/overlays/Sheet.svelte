<script lang="ts">
	import { modal } from '$lib/a11y/modal';
	import type { Snippet } from 'svelte';

	let {
		children,
		grip = true,
		label,
		ondismiss,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		grip?: boolean;
		/** Имя для читалки (`aria-label`). */
		label?: string;
		/** Задан — лист модален: фокус внутри, Escape закрывает (UI-1). */
		ondismiss?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div
	class="sheet {className}"
	{style}
	role={ondismiss ? 'dialog' : undefined}
	aria-modal={ondismiss ? 'true' : undefined}
	aria-label={label}
	use:modal={{ ondismiss }}
>
	{#if grip}
		<div class="grip"></div>
	{/if}
	{@render children()}
</div>
