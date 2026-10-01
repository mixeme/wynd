<script lang="ts">
	import Input from '$ui/forms/Input.svelte';

	// Число «Своё…» с единицей справа (план 47, 2.9): часы окна правок,
	// гигабайты кэша и квоты, люди и дни приглашения. Поле в 72 px и подпись
	// были скопированы в шесть мест — на служебных классах и на инлайн-стилях;
	// служебные проигрывали input.fld, ширину держит input.fld.num.
	let {
		value = $bindable(),
		min,
		max,
		unit,
		onchange,
		class: className = ''
	}: {
		value: number;
		min?: number;
		max?: number;
		/** Что за число и его предел: «часов», «ГБ, до 100». */
		unit: string;
		/** Значение подтвердили (уход из поля, Enter) — сохранить. */
		onchange?: () => void;
		class?: string;
	} = $props();
</script>

<div class="rowin mt-10 {className}">
	<Input
		active
		type="number"
		min={min === undefined ? undefined : String(min)}
		max={max === undefined ? undefined : String(max)}
		class="num"
		bind:value
		onchange={() => onchange?.()}
	/>
	<span class="hint m-0">{unit}</span>
</div>
