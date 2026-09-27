<script lang="ts">
	import { onMount } from 'svelte';

	let {
		value = $bindable<string | undefined>(undefined),
		digits: digitsProp = [],
		length = 6,
		active: activeProp = 0,
		el = $bindable<HTMLInputElement>(),
		autofocus = false,
		'aria-label': inputLabel = 'Код из шести цифр'
	}: {
		value?: string;
		digits?: string[];
		length?: number;
		active?: number;
		el?: HTMLInputElement;
		autofocus?: boolean;
		'aria-label'?: string;
	} = $props();

	const editable = $derived(value !== undefined);

	const cleanValue = $derived(
		editable ? (value ?? '').replace(/\D/g, '').slice(0, length) : ''
	);

	$effect(() => {
		if (!editable || value === undefined) return;
		if (cleanValue !== value) value = cleanValue;
	});

	const digits = $derived(editable ? cleanValue.split('') : digitsProp);
	// Код набран целиком — курсора нет: в последней клетке он вставал второй
	// строкой под цифрой.
	const active = $derived(
		editable ? (cleanValue.length < length ? cleanValue.length : -1) : activeProp
	);

	const cells = $derived(
		Array.from({ length }, (_, i) => ({
			char: digits[i] ?? '',
			active: i === active
		}))
	);

	onMount(() => {
		if (autofocus && editable) {
			queueMicrotask(() => el?.focus());
		}
	});
</script>

<div class="code-wrap">
	{#if editable}
		<!-- Без maxlength: браузер резал вставку «123 456» до шести символов
		     раньше, чем cleanValue выбрасывал пробел, и терялась цифра. -->
		<input
			bind:this={el}
			class="code-input"
			type="text"
			inputmode="numeric"
			autocomplete="one-time-code"
			aria-label={inputLabel}
			bind:value
		/>
	{/if}
	<div class="codebox">
		{#each cells as cell, i (i)}
			<u class:act={cell.active}>{cell.char}</u>
		{/each}
	</div>
</div>
