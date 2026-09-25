<script lang="ts">
	let {
		checked = $bindable(false),
		disabled = false,
		label = 'Переключить',
		class: className = '',
		style = ''
	}: {
		checked?: boolean;
		disabled?: boolean;
		label?: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<button
	type="button"
	role="switch"
	aria-checked={checked}
	aria-label={label}
	class="sw {className}"
	class:on={checked}
	{style}
	{disabled}
	style:opacity={disabled ? 0.5 : undefined}
	onclick={() => {
		if (!disabled) checked = !checked;
	}}
></button>
