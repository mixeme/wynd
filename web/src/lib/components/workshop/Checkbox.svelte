<script lang="ts">
	import type { Snippet } from 'svelte';

	// Галочка окна Мастерской (план 54) — нарисованная, в цвете Wynd (макет
	// installer.html, .cbx; владелец 2026-10-24: «Галочки у нас какие-то
	// страшные»). Подпись — children; пояснение в ней — <span class="ins-dim">.
	// Стили — .ins-check в styles/workshop.css: общие с Radio.
	let {
		checked = $bindable(false),
		disabled = false,
		children,
		class: className = '',
		style = ''
	}: {
		checked?: boolean;
		disabled?: boolean;
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<label class="ins-check {className}" {style}>
	<input type="checkbox" bind:checked {disabled} />
	<span>{@render children()}</span>
</label>
