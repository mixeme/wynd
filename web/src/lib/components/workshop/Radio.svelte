<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';

	// Выбор одного в окне Мастерской (план 54) — нарисованный, как Checkbox.
	// bind:group через компонент не проходит: выбранное — в group, своё — value.
	// Стили — .ins-check в styles/workshop.css.
	let {
		group = $bindable(),
		value,
		name,
		disabled = false,
		children,
		class: className = '',
		style = ''
	}: {
		group: T;
		value: T;
		name: string;
		disabled?: boolean;
		children: Snippet;
		class?: string;
		style?: string;
	} = $props();
</script>

<label class="ins-check {className}" {style}>
	<input
		type="radio"
		{name}
		value={String(value)}
		checked={group === value}
		{disabled}
		onchange={() => (group = value)}
	/>
	<span>{@render children()}</span>
</label>
