<script lang="ts">
	import {
		CIRCLE_COLOR_ORDER,
		CIRCLE_COLORS,
		type CircleColor
	} from '$lib/theme/colors';

	let {
		value = $bindable<CircleColor>('ochre'),
		class: className = ''
	}: {
		value?: CircleColor;
		class?: string;
	} = $props();
</script>

<div class="sws {className}">
	{#each CIRCLE_COLOR_ORDER as color (color)}
		<button
			type="button"
			aria-label={color}
			aria-pressed={value === color}
			class:on={value === color}
			style:background={CIRCLE_COLORS[color].hex}
			onclick={() => (value = color)}
		></button>
	{/each}
</div>
