<script lang="ts">
	let {
		checked = $bindable(false),
		disabled = false,
		class: className = '',
		style = ''
	}: {
		checked?: boolean;
		disabled?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<button
	type="button"
	role="switch"
	aria-checked={checked}
	class="sw {className}"
	class:on={checked}
	{style}
	{disabled}
	style:opacity={disabled ? 0.5 : undefined}
	onclick={() => {
		if (!disabled) checked = !checked;
	}}
></button>
