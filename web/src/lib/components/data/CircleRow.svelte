<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	let {
		initial,
		name,
		preview,
		time,
		badge,
		dot = false,
		silent,
		color,
		card = false,
		actionLabel,
		onaction,
		actionLabel2,
		onaction2,
		onclick,
		onmousedown,
		onmouseup,
		onmouseleave,
		ontouchstart,
		ontouchend,
		ontouchcancel,
		class: className = '',
		style = ''
	}: {
		initial: string;
		name: string;
		preview: string;
		time: string;
		badge?: number | string;
		/** Новые отклики без новых записей (2.1): точка, а не число. */
		dot?: boolean;
		/** Сервер круга молчит (7.9): вместо последней строки — какой, круг приглушён. */
		silent?: string;
		color?: string;
		card?: boolean;
		actionLabel?: string;
		onaction?: () => void;
		actionLabel2?: string;
		onaction2?: () => void;
		onclick?: () => void;
		onmousedown?: (e: MouseEvent) => void;
		onmouseup?: (e: MouseEvent) => void;
		onmouseleave?: (e: MouseEvent) => void;
		ontouchstart?: (e: TouchEvent) => void;
		ontouchend?: (e: TouchEvent) => void;
		ontouchcancel?: (e: TouchEvent) => void;
		class?: string;
		style?: string;
	} = $props();

	const rowStyle = $derived(color ? `--rc:${color};${style}` : style);
	const showAction = $derived(card && actionLabel && onaction);
	const showAction2 = $derived(card && actionLabel2 && onaction2);
</script>

<!-- Содержимое строки — один сниппет вместо четырёх копий (карточка ×
     нажимаемая); с onclick строка — <button> с жестами длинного нажатия,
     без — <div> (план 42, UI-5). -->
{#snippet inner()}
	<div class="sq">{initial}</div>
	<div class="m">
		<div class="n">{name}</div>
		{#if silent}
			<div class="p with-ic"><Icon name="clock" size="xs" />{silent} не отвечает</div>
		{:else}
			<div class="p">{preview}</div>
		{/if}
	</div>
	<div class="rt">
		<span>{time}</span>
		{#if badge !== undefined}
			<span class="bdg">{badge}</span>
		{:else if dot}
			<span class="rdot" role="img" aria-label="новые отклики"></span>
		{/if}
	</div>
{/snippet}

{#snippet row(rowClass: string)}
	{#if onclick}
		<button
			type="button"
			class={rowClass}
			style={rowStyle}
			{onclick}
			{onmousedown}
			{onmouseup}
			{onmouseleave}
			{ontouchstart}
			{ontouchend}
			{ontouchcancel}
		>
			{@render inner()}
		</button>
	{:else}
		<div class={rowClass} style={rowStyle}>
			{@render inner()}
		</div>
	{/if}
{/snippet}

{#if card}
	<div class="circle-row-card {className}">
		{@render row(silent ? 'r silent' : 'r')}
		{#if showAction}
			<button type="button" class="circle-row-action" onclick={() => onaction?.()}>
				{actionLabel}
			</button>
		{/if}
		{#if showAction2}
			<button type="button" class="circle-row-action" onclick={() => onaction2?.()}>
				{actionLabel2}
			</button>
		{/if}
	</div>
{:else}
	{@render row(`r ${silent ? 'silent ' : ''}${className}`)}
{/if}
