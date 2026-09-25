<script lang="ts">
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';

	export interface CircleRowGroupChip {
		label: string;
		selected?: boolean;
		onclick: () => void;
	}

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
		groupChips,
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
		groupChips?: CircleRowGroupChip[];
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
	const cardWrapStyle =
		'margin:8px 10px;background:var(--card);border:1px solid var(--line);border-radius:14px';
	const showAction = $derived(card && actionLabel && onaction);
	const showGroupChips = $derived(card && groupChips && groupChips.length > 0);
</script>

{#if card}
	<div class="circle-row-card {className}" style={cardWrapStyle}>
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
		{#if showGroupChips}
			<ChipGroup style="margin:0 0 14px;padding:0 16px">
				{#each groupChips as chip (chip.label)}
					<Chip selected={chip.selected} onclick={chip.onclick}>{chip.label}</Chip>
				{/each}
			</ChipGroup>
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
