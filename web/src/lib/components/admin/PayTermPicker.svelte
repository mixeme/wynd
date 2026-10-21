<script lang="ts">
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Input from '$ui/forms/Input.svelte';

	// Срок подписки (10.12, 10.15 и включение подписки, 2026-10-21): одни и те
	// же чипы везде, где администратор выдаёт срок.
	const DAY_CHIPS = [
		{ days: 30, label: '30 дней' },
		{ days: 90, label: '3 месяца' },
		{ days: 365, label: 'Год' }
	] as const;

	let {
		days = $bindable(90),
		custom = $bindable(false),
		customDays = $bindable(''),
		unlimited = $bindable(false),
		none = $bindable(false),
		noneLabel
	}: {
		days?: number;
		custom?: boolean;
		customDays?: string;
		unlimited?: boolean;
		/** Не выдавать срок — только там, где это выбор («Не давать»). */
		none?: boolean;
		noneLabel?: string;
	} = $props();

	function pick(next: { days?: number; custom?: boolean; unlimited?: boolean; none?: boolean }) {
		custom = next.custom ?? false;
		unlimited = next.unlimited ?? false;
		none = next.none ?? false;
		if (next.days != null) days = next.days;
	}
</script>

<ChipGroup class="m-0">
	{#each DAY_CHIPS as chip (chip.days)}
		<Chip selected={!unlimited && !custom && !none && days === chip.days} onclick={() => pick({ days: chip.days })}>
			{chip.label}
		</Chip>
	{/each}
	<Chip selected={custom && !unlimited && !none} onclick={() => pick({ custom: true })}>Своё…</Chip>
	<!-- Одной группой: в широкой колонке — одна строка, в узкой переносится сама. -->
	<Chip selected={unlimited} onclick={() => pick({ unlimited: true })}>Бессрочно</Chip>
	{#if noneLabel}
		<Chip selected={none} onclick={() => pick({ none: true })}>{noneLabel}</Chip>
	{/if}
</ChipGroup>
{#if custom && !unlimited && !none}
	<Input admin type="number" placeholder="дней" class="mt-10 w120" bind:value={customDays} />
{/if}
