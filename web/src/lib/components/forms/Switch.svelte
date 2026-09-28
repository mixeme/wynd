<script lang="ts">
	let {
		checked = $bindable(false),
		disabled = false,
		label = 'Переключить',
		onchange,
		class: className = '',
		style = ''
	}: {
		checked?: boolean;
		disabled?: boolean;
		label?: string;
		// Только нажатие человеком — не смена checked извне (сохранение на
		// сервер не должно срабатывать от обновления карточки).
		onchange?: (checked: boolean) => void;
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
		if (disabled) return;
		checked = !checked;
		onchange?.(checked);
	}}
></button>
