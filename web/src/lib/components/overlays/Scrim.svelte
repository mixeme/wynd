<script lang="ts">
	let {
		onclick,
		class: className = '',
		style = ''
	}: {
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="scrim {className}" {style} {onclick}></div>
