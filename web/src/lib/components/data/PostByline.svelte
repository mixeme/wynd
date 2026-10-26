<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import Icon, { type IconName } from '$ui/Icon.svelte';

	// Кто и когда — левая часть шапки карточки записи (3.1, 4.5, 5.2; план 47,
	// 1.8): аватар, имя, время. Был собран от руки в ленте, очереди, дне и на
	// экране записи. Для snippet `author` у PostCard.
	let {
		initial,
		color,
		src,
		name,
		time,
		icon,
		day
	}: {
		initial: string;
		color?: string;
		src?: string;
		name: string;
		/** «вчера, 20:40» или «в очереди». */
		time: string;
		/** Значок перед временем: часы у записи в очереди. */
		icon?: IconName;
		/** День записи, если он не день публикации, — строкой под временем
		 * (экран записи: справа в шапке уже правка и обложка). */
		day?: string;
	} = $props();
</script>

<Avatar {initial} {color} {src} />
<div>
	<div class="n">{name}</div>
	<div class="tm" class:tm-icon={Boolean(icon)}>
		{#if icon}<Icon name={icon} size="xs" />{/if}{time}
	</div>
	{#if day}<div class="tm tm-icon"><Icon name="day" size="xs" />{day}</div>{/if}
</div>
