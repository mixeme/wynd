<script lang="ts">
	import PayGateLayout from '$lib/layouts/PayGateLayout.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<PayGateLayout>{@render children()}</PayGateLayout>
