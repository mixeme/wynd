<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';

	let {
		name,
		label,
		onclick,
		size,
		stopPropagation = false,
		class: className = '',
		style = '',
		el = $bindable<HTMLButtonElement>()
	}: {
		name: IconName;
		label: string;
		onclick: () => void;
		size?: 'md' | 'sm' | 'xs';
		stopPropagation?: boolean;
		class?: string;
		style?: string;
		el?: HTMLButtonElement;
	} = $props();

	function onClick(e: MouseEvent) {
		if (stopPropagation) e.stopPropagation();
		onclick();
	}
</script>

<button type="button" class="ib {className}" {style} aria-label={label} bind:this={el} onclick={onClick}>
	<Icon {name} {size} />
</button>
