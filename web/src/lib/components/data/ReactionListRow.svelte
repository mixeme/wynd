<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';

	let {
		initial,
		name,
		color,
		emoji
	}: {
		initial: string;
		name: string;
		color?: string;
		emoji: string;
	} = $props();
</script>

<div class="row2">
	<Avatar {initial} {color} />
	<div class="g" style="font-weight:600">{name}</div>
	<span class="rxe rxe-row">{emoji}</span>
</div>
