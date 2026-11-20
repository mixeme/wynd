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

	function openOnTouch(event: MouseEvent & { currentTarget: HTMLInputElement }) {
		if (!window.matchMedia('(pointer: coarse)').matches) return;
		try {
			event.currentTarget.showPicker();
			event.preventDefault();
		} catch {
			// Старый браузер открывает календарь своим значком.
		}
	}
</script>

<div class="date-range">
	<label class="date-range-field" class:dated={Boolean(from)}>
		{#if !from}<span class="date-range-prefix" aria-hidden="true">с</span>{/if}
		<Input type="date" bind:value={from} active aria-label="с" onclick={openOnTouch} />
		<span class="date-range-icon" aria-hidden="true"><Icon name="day" size="sm" /></span>
		<span class="date-range-chevron" aria-hidden="true"><Icon name="chev" size="sm" /></span>
	</label>
	<label class="date-range-field" class:dated={Boolean(to)}>
		{#if !to}<span class="date-range-prefix" aria-hidden="true">по</span>{/if}
		<Input type="date" bind:value={to} active aria-label="по" onclick={openOnTouch} />
		<span class="date-range-icon" aria-hidden="true"><Icon name="day" size="sm" /></span>
		<span class="date-range-chevron" aria-hidden="true"><Icon name="chev" size="sm" /></span>
	</label>
</div>
