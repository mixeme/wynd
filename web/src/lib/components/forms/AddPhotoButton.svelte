<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let { onclick }: { onclick: () => void } = $props();
</script>

<button type="button" class="addph" {onclick}>
	<Icon name="plus" />
</button>
