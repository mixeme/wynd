<script lang="ts">
	import Input from '$ui/forms/Input.svelte';
	import Icon from '$ui/Icon.svelte';

	// Период «с — по» двумя полями даты (2.19 «Период»; план 47, 2.11): поиск
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
	<label class="date-range-field">
		<span class="date-range-prefix" aria-hidden="true">с</span>
		<Input type="date" bind:value={from} active aria-label="с" />
		<span class="date-range-chevron" aria-hidden="true"><Icon name="chev" size="sm" /></span>
	</label>
	<label class="date-range-field">
		<span class="date-range-prefix" aria-hidden="true">по</span>
		<Input type="date" bind:value={to} active aria-label="по" />
		<span class="date-range-chevron" aria-hidden="true"><Icon name="chev" size="sm" /></span>
	</label>
</div>
