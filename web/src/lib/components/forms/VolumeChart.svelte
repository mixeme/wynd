<script lang="ts">
	import type { VolumeBucket } from '$lib/circles/settings';

	let {
		volume = [] as VolumeBucket[],
		cutoffLabel = '',
		cutoffX = 205,
		class: className = ''
	}: {
		volume?: VolumeBucket[];
		cutoffLabel?: string;
		cutoffX?: number;
		class?: string;
	} = $props();

	const chartWidth = 358;
	const chartHeight = 132;
	const barAreaTop = 16;
	const barAreaHeight = 88;
	const baselineY = 104;

	const maxBytes = $derived(Math.max(...volume.map((b) => b.bytes), 1));
	const barWidth = 18;
	const barGap = 8;
	const barStep = barWidth + barGap;

	const bars = $derived(
		volume.map((b, i) => {
			const h = Math.max(4, Math.round((b.bytes / maxBytes) * barAreaHeight));
			return {
				x: 1 + i * barStep,
				y: baselineY - h,
				w: barWidth,
				h,
				beforeCutoff: i * barStep + barWidth / 2 <= cutoffX
			};
		})
	);

	const yearStart = $derived(volume[0]?.period.slice(0, 4) ?? '');
	const yearEnd = $derived(volume[volume.length - 1]?.period.slice(0, 4) ?? '');

	const cutoffLine = $derived(`M${cutoffX} 10 V112`);
	const shadedWidth = $derived(Math.max(0, cutoffX));
</script>

<div class="chart {className}">
	<svg viewBox="0 0 {chartWidth} {chartHeight}" aria-hidden="true">
		{#if volume.length}
			<rect x="0" y={barAreaTop} width={shadedWidth} height={barAreaHeight} fill="var(--ct)" />
			{#each bars as bar (bar.x)}
				<rect
					x={bar.x}
					y={bar.y}
					width={bar.w}
					height={bar.h}
					rx="2"
					fill={bar.beforeCutoff ? 'var(--c)' : '#C9C0B2'}
				/>
			{/each}
		{/if}
		<path d="M0 {baselineY} H{chartWidth}" stroke="#DFD8CD" stroke-width="1" fill="none" />
		{#if cutoffLabel}
			<path d={cutoffLine} stroke="var(--c)" stroke-width="1.5" fill="none" />
			<text x={cutoffX} y="126" text-anchor="middle" font-size="10" fill="var(--c)"
				>{cutoffLabel}</text
			>
		{/if}
		{#if yearStart}
			<text x="2" y="126" font-size="10" fill="#A8A096">{yearStart}</text>
		{/if}
		{#if yearEnd && yearEnd !== yearStart}
			<text x="330" y="126" font-size="10" fill="#A8A096">{yearEnd}</text>
		{/if}
	</svg>
</div>

<style>
	.chart {
		margin: 0 16px;
	}
	.chart svg {
		display: block;
		width: 100%;
		height: auto;
	}
</style>
