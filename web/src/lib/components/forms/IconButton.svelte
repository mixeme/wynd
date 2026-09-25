<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';

	let {
		name,
		label,
		onclick,
		size,
		disabled = false,
		stopPropagation = false,
		class: className = '',
		style = '',
		el = $bindable<HTMLButtonElement>()
	}: {
		name: IconName;
		label: string;
		onclick: () => void;
		size?: 'md' | 'sm' | 'xs';
		disabled?: boolean;
		stopPropagation?: boolean;
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
	{style}
	{disabled}
	aria-label={label}
	bind:this={el}
	onclick={onClick}
>
	<Icon {name} {size} />
</button>
