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

<button type="button" class="scrim {className}" {style} aria-label="Закрыть" {onclick}></button>
