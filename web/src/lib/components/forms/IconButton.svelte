<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';

	let {
		name,
		label,
		onclick,
		size,
		disabled = false,
		stopPropagation = false,
		onpointerdown,
		onpointerup,
		onpointerleave,
		onpointercancel,
		pressed = undefined,
		class: className = '',
		style = '',
		el = $bindable<HTMLButtonElement>()
	}: {
		name: IconName;
		label: string;
		onclick: () => void;
		size?: 'md' | 'sm' | 'xs';
		disabled?: boolean;
		// Переключатель: true — включено (цвет круга), false — выключено
		// (приглушён). Не задан — обычная кнопка без aria-pressed.
		pressed?: boolean;
		stopPropagation?: boolean;
		onpointerdown?: (e: PointerEvent) => void;
		onpointerup?: (e: PointerEvent) => void;
		onpointerleave?: (e: PointerEvent) => void;
		onpointercancel?: (e: PointerEvent) => void;
		class?: string;
		style?: string;
		el?: HTMLButtonElement;
	} = $props();

	function onClick(e: MouseEvent) {
		if (disabled) return;
		if (stopPropagation) e.stopPropagation();
		onclick();
	}
</script>

<button
	type="button"
	class="ib {className}"
	class:off={pressed === false}
	aria-pressed={pressed}
	{style}
	{disabled}
	aria-label={label}
	bind:this={el}
	onclick={onClick}
	{onpointerdown}
	{onpointerup}
	{onpointerleave}
	{onpointercancel}
>
	<Icon {name} {size} />
</button>
