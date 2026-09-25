<script lang="ts">
	import CheckRow from '$ui/admin/CheckRow.svelte';
	import Button from '$ui/forms/Button.svelte';

	let {
		circleColor,
		circleName,
		requested,
		previous,
		requester,
		date,
		freeSpace,
		onapprove,
		onreject,
		class: className = '',
		style = ''
	}: {
		circleColor: string;
		circleName: string;
		requested: string;
		previous?: string;
		requester: string;
		date: string;
		freeSpace: string;
		onapprove?: () => void;
		onreject?: () => void;
		class?: string;
		style?: string;
	} = $props();

	const headline = $derived(
		previous ? `${circleName} · ${requested} вместо ${previous}` : `${circleName} · ${requested}`
	);
</script>

<CheckRow
	status="warn"
	class={className}
	{style}
	description="попросил {requester} {date}; свободно {freeSpace}"
>
	{#snippet title()}
		<span
			style="width:12px;height:12px;border-radius:4px;display:inline-block;vertical-align:-1px;margin-right:8px;background:{circleColor}"
		></span>{headline}
	{/snippet}
	{#snippet actions()}
		{#if onapprove}
			<Button style="margin-right:10px" onclick={() => onapprove()}>Дать</Button>
		{/if}
		{#if onreject}
			<Button variant="ghost" onclick={() => onreject()}>Отказать</Button>
		{/if}
	{/snippet}
</CheckRow>
