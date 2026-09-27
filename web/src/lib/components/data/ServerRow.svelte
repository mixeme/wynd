<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import Row from '$ui/data/Row.svelte';

	type ServerRowVariant = 'select' | 'ok' | 'warn' | 'info';

	let {
		name,
		subtitle,
		variant = 'info',
		card = false,
		onclick,
		class: className = '',
		style = ''
	}: {
		name: string;
		subtitle: string;
		variant?: ServerRowVariant;
		card?: boolean;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();

	// width:auto — у .row2 стоит width:100%, и с полями карточки по 16 px она
	// вылезала за правый край на 32 px (экран приглашения 1.7).
	const cardStyle = $derived(
		card
			? 'margin:14px 16px 0;width:auto;border:1px solid var(--line);background:var(--card);border-radius:12px;padding:12px 14px'
			: ''
	);
	const rowStyle = $derived(cardStyle ? `${cardStyle};${style}` : style);
</script>

<Row {onclick} class={className} style={rowStyle}>
	{#snippet leading()}<Icon name="cloud" />{/snippet}
	{#snippet main()}
		<div style="font-weight:600">{name}</div>
		<div class="sub">{subtitle}</div>
	{/snippet}
	{#snippet trailing()}
		{#if variant === 'select'}
			<Icon name="chevr" size="sm" />
		{:else if variant === 'ok'}
			<span class="st ok"><Icon name="check" /></span>
		{:else if variant === 'warn'}
			<span class="st warn">!</span>
		{/if}
	{/snippet}
</Row>
