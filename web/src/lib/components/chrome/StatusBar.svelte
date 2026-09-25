<script lang="ts">
	let { time = '9:41', dim = false }: { time?: string; dim?: boolean } = $props();
</script>

<div class="sb">
	<span>{time}</span>
	<span class="sig" class:dim><u></u><u></u><u></u><u></u></span>
</div>
