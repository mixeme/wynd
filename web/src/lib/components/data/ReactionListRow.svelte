<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import Icon, { type IconName } from '$ui/Icon.svelte';

	let {
		initial,
		name,
		color,
		icon
	}: {
		initial: string;
		name: string;
		color?: string;
		icon: IconName;
	} = $props();
</script>

<div class="row2">
	<Avatar {initial} {color} />
	<div class="g" style="font-weight:600">{name}</div>
	<Icon name={icon} size="sm" style="color:var(--c)" />
</div>
