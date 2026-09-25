<script lang="ts">
	import Icon from '$ui/Icon.svelte';

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

	const cardStyle = $derived(
		card
			? 'margin:14px 16px 0;border:1px solid var(--line);background:var(--card);border-radius:12px;padding:12px 14px'
			: ''
	);
	const rowStyle = $derived(cardStyle ? `${cardStyle};${style}` : style);
</script>

{#if onclick}
	<button type="button" class="row2 {className}" style={rowStyle} {onclick}>
		<Icon name="cloud" />
		<div class="g">
			<div style="font-weight:600">{name}</div>
			<div class="sub">{subtitle}</div>
		</div>
		{#if variant === 'select'}
			<Icon name="chevr" size="sm" />
		{:else if variant === 'ok'}
			<span class="st ok"><Icon name="check" /></span>
		{:else if variant === 'warn'}
			<span class="st warn">!</span>
		{/if}
	</button>
{:else}
	<div class="row2 {className}" style={rowStyle}>
		<Icon name="cloud" />
		<div class="g">
			<div style="font-weight:600">{name}</div>
			<div class="sub">{subtitle}</div>
		</div>
		{#if variant === 'select'}
			<Icon name="chevr" size="sm" />
		{:else if variant === 'ok'}
			<span class="st ok"><Icon name="check" /></span>
		{:else if variant === 'warn'}
			<span class="st warn">!</span>
		{/if}
	</div>
{/if}
