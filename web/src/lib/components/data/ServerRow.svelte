<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import Row from '$ui/data/Row.svelte';

	type ServerRowVariant = 'select' | 'ok' | 'warn' | 'info';

	let {
		name,
		subtitle,
		detail,
		variant = 'info',
		card = false,
		gdpr = false,
		top = 14,
		onclick,
		class: className = '',
		style = ''
	}: {
		name: string;
		subtitle: string;
		/** Третья строка: сведения по контексту или состояние связи (план 56). */
		detail?: string;
		variant?: ServerRowVariant;
		card?: boolean;
		/**
		 * Значок «GDPR» у имени (1.2): администратор разрешил уходящим стирать
		 * имена и фото (9.4). Свойство сервера — стоит везде, где виден сервер.
		 */
		gdpr?: boolean;
		/** Верхний отступ карточки, px: 0 — сразу под подписью поля («Сервер» на 1.1). */
		top?: number;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();

	// width:auto — у .row2 стоит width:100%, и с полями карточки по 16 px она
	// вылезала за правый край на 32 px (экран приглашения 1.8).
	const cardStyle = $derived(
		card
			? `margin:${top}px 16px 0;width:auto;border:1px solid var(--line);background:var(--card);border-radius:12px;padding:12px 14px`
			: ''
	);
	const rowStyle = $derived(cardStyle ? `${cardStyle};${style}` : style);
</script>

<Row {onclick} class={card ? `server-card ${className}`.trim() : className} style={rowStyle}>
	{#snippet leading()}<Icon name="cloud" />{/snippet}
	{#snippet main()}
		<div style="font-weight:600">
			{name}{#if gdpr}<span class="gdpr">GDPR</span>{/if}
		</div>
		<div class="sub">{subtitle}</div>
		{#if detail}
			<div class="sub">{detail}</div>
		{/if}
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
