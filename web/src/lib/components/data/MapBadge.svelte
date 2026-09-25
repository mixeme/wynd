<script lang="ts">
	let {
		badge,
		class: className = '',
		style = ''
	}: {
		badge: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="map-badge {className}" {style}>{badge}</div>
