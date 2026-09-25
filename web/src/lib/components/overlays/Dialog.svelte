<script lang="ts">
	import { modal } from '$lib/a11y/modal';
	import type { Snippet } from 'svelte';

	let {
		children,
		label,
		ondismiss,
		class: className = '',
		style = ''
	}: {
		children: Snippet;
		/** Имя для читалки (`aria-label`). */
		label?: string;
		/** Задан — диалог модален: фокус внутри, Escape закрывает (UI-1). */
		ondismiss?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div
	class="dlg {className}"
	{style}
	role={ondismiss ? 'dialog' : undefined}
	aria-modal={ondismiss ? 'true' : undefined}
	aria-label={label}
	use:modal={{ ondismiss }}
>
	{@render children()}
</div>
