<script lang="ts">
	let {
		initial,
		name,
		preview,
		time,
		badge,
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

{#if card}
	<div class="circle-row-card {className}">
		{#if onclick}
			<button
				type="button"
				class="r"
				style={rowStyle}
				{onclick}
				{onmousedown}
				{onmouseup}
				{onmouseleave}
				{ontouchstart}
				{ontouchend}
				{ontouchcancel}
			>
				<div class="sq">{initial}</div>
				<div class="m">
					<div class="n">{name}</div>
					<div class="p">{preview}</div>
				</div>
				<div class="rt">
					<span>{time}</span>
					{#if badge !== undefined}
						<span class="bdg">{badge}</span>
					{/if}
				</div>
			</button>
		{:else}
			<div class="r" style={rowStyle}>
				<div class="sq">{initial}</div>
				<div class="m">
					<div class="n">{name}</div>
					<div class="p">{preview}</div>
				</div>
				<div class="rt">
					<span>{time}</span>
					{#if badge !== undefined}
						<span class="bdg">{badge}</span>
					{/if}
				</div>
			</div>
		{/if}
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
{:else if onclick}
	<button
		type="button"
		class="r {className}"
		style={rowStyle}
		{onclick}
		{onmousedown}
		{onmouseup}
		{onmouseleave}
		{ontouchstart}
		{ontouchend}
		{ontouchcancel}
	>
		<div class="sq">{initial}</div>
		<div class="m">
			<div class="n">{name}</div>
			<div class="p">{preview}</div>
		</div>
		<div class="rt">
			<span>{time}</span>
			{#if badge !== undefined}
				<span class="bdg">{badge}</span>
			{/if}
		</div>
	</button>
{:else}
	<div class="r {className}" style={rowStyle}>
		<div class="sq">{initial}</div>
		<div class="m">
			<div class="n">{name}</div>
			<div class="p">{preview}</div>
		</div>
		<div class="rt">
			<span>{time}</span>
			{#if badge !== undefined}
				<span class="bdg">{badge}</span>
			{/if}
		</div>
	</div>
{/if}
