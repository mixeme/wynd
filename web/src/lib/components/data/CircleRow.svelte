<script lang="ts">
	let {
		initial,
		name,
		preview,
		time,
		badge,
		color,
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
</script>

{#if onclick}
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
