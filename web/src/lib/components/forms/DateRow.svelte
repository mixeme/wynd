<script lang="ts">
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import type { IconName } from '$ui/Icon.svelte';

	// Строка с датой, которая открывает системный выбор дня (4.2 «Отнести к
	// дате»; план 47, 3.7). Само поле даты скрыто под строкой: выбор открывается
	// у неё, а не в углу экрана.
	let {
		value = $bindable(''),
		title,
		subtitle,
		icon,
		style = ''
	}: {
		/** «2026-10-01». */
		value?: string;
		title: string;
		subtitle?: string;
		icon?: IconName;
		style?: string;
	} = $props();

	let input: HTMLInputElement | undefined = $state();

	function open() {
		if (!input) return;
		try {
			input.showPicker();
		} catch {
			// Старые браузеры без showPicker — выбор по клику.
			input.click();
		}
	}
</script>

<div class="date-row">
	<input bind:this={input} type="date" bind:value class="date-pick" tabindex="-1" />
	<SettingsRow {icon} {title} {subtitle} {style} onclick={open} />
</div>
