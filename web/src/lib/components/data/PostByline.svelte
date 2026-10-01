<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import Icon, { type IconName } from '$ui/Icon.svelte';

	// Кто и когда — левая часть шапки карточки записи (3.1, 4.4, 5.2; план 47,
	// 1.7): аватар, имя, время. Был собран от руки в ленте, очереди, дне и на
	// экране записи. Для snippet `author` у PostCard.
	let {
		initial,
		color,
		src,
		name,
		time,
		icon
	}: {
		initial: string;
		color?: string;
		src?: string;
		name: string;
		/** «вчера, 20:40» или «в очереди». */
		time: string;
		/** Значок перед временем: часы у записи в очереди. */
		icon?: IconName;
	} = $props();
</script>

<Avatar {initial} {color} {src} />
<div>
	<div class="n">{name}</div>
	<div class="tm" class:tm-icon={Boolean(icon)}>{#if icon}<Icon name={icon} size="xs" />{/if}{time}</div>
</div>
