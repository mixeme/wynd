<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';

	// Отметка справа в шапке карточки: день, к которому привязана запись
	// (значок календаря), или «внесено сегодня» (часы; план 47, 1.7).

	let {
		label,
		icon = 'day',
		class: className = '',
		style = ''
	}: {
		label: string;
		icon?: IconName;
		class?: string;
		style?: string;
	} = $props();
</script>

<span
	class="tm {className}"
	{style}
	style:margin-left="auto"
	style:display="flex"
	style:align-items="center"
	style:gap="5px"
>
	<Icon name={icon} size="xs" />
	{label}
</span>
