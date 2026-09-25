<script lang="ts">
	let {
		value,
		max = 100
	}: {
		value: number;
		max?: number;
	} = $props();

	const width = $derived(`${Math.min(100, Math.max(0, (value / max) * 100))}%`);
</script>

<div class="meter">
	<u style:width={width}></u>
</div>
