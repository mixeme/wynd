<script lang="ts">
	let {
		initial,
		name,
		preview
	}: {
		initial: string;
		name: string;
		preview: string;
	} = $props();
</script>

<div class="icard">
	<div class="sq">{initial}</div>
	<div class="m">
		<div class="n">{name}</div>
		<div class="p">{preview}</div>
	</div>
</div>
