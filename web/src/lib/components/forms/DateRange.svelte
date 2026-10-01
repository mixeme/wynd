<script lang="ts">
	import Input from '$ui/forms/Input.svelte';

	// Период «с — по» двумя полями даты (2.9 «Период»; план 47, 2.11): поиск
	// в круге и по всем кругам.
	let {
		from = $bindable(''),
		to = $bindable('')
	}: {
		/** «2026-09-01»; пусто — без нижней границы. */
		from?: string;
		to?: string;
	} = $props();
</script>

<div class="date-range">
	<Input type="date" bind:value={from} active placeholder="с" aria-label="с" />
	<Input type="date" bind:value={to} active placeholder="по" aria-label="по" />
</div>
