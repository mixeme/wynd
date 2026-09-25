<script lang="ts">
	let {
		class: className = '',
		style = ''
	}: {
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="scrim {className}" {style}></div>
