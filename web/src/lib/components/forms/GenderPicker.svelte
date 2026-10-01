<script lang="ts">
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Label from '$ui/forms/Label.svelte';

	// Пол для строк журнала (план 46, A5): «Аня вступила в круг». Прямо,
	// без пояснений. Не выбран — строки без глагола. Повторное касание
	// выбранного снимает выбор.
	let { value = $bindable('') }: { value?: '' | 'm' | 'f' } = $props();

	function pick(next: 'm' | 'f') {
		value = value === next ? '' : next;
	}
</script>

<Label>Пол</Label>
<ChipGroup>
	<Chip selected={value === 'm'} onclick={() => pick('m')}>Мужской</Chip>
	<Chip selected={value === 'f'} onclick={() => pick('f')}>Женский</Chip>
</ChipGroup>
