<script lang="ts">
	import type { Snippet } from 'svelte';

	// Галочка окна Мастерской (план 54) — нарисованная, в цвете Wynd (макет
	// installer.html, .cbx; владелец 2026-10-24: «Галочки у нас какие-то
	// страшные»). Подпись — children; пояснение в ней — <span class="ins-dim">.
	// Стили — .ins-check в styles/workshop.css: общие с Radio.
	let {
		checked = $bindable(false),
		disabled = false,
		native = false,
		onchange,
		children,
		class: className = '',
		style = ''
	}: {
		checked?: boolean;
		disabled?: boolean;
		native?: boolean;
		onchange?: (checked: boolean) => void;
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<label class="ins-check {className}" class:ins-native={native} {style}>
	<input
		type="checkbox"
		bind:checked
		{disabled}
		onchange={(event) => onchange?.(event.currentTarget.checked)}
	/>
	<span>{@render children()}</span>
</label>

<style>
	.ins-check.ins-native {
		gap: 8px;
		font-size: 13px;
		font-weight: 400;
	}
	.ins-check.ins-native input {
		appearance: auto;
		display: block;
		flex: 0 0 16px;
		width: 16px;
		height: 16px;
		margin: 2px 0 0;
		border: initial;
		border-radius: initial;
		background: initial;
		accent-color: var(--c);
	}
	.ins-check.ins-native input:checked::after {
		content: none;
	}
</style>
