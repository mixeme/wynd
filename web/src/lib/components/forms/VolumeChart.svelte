<script lang="ts">
	import type { VolumeBucket } from '$lib/circles/settings';
	import {
		VOLUME_BAR_WIDTH,
		VOLUME_CHART_WIDTH,
		volumeBarCenterX,
		volumeBarX
	} from '$lib/circles/volumeChart';

	let {
		volume = [] as VolumeBucket[],
		cutoffLabel = '',
		cutoffX = $bindable(205),
		oncutoff,
		class: className = ''
	}: {
		volume?: VolumeBucket[];
		cutoffLabel?: string;
		cutoffX?: number;
		oncutoff?: (index: number) => void;
		class?: string;
	} = $props();

	const chartWidth = VOLUME_CHART_WIDTH;
	const chartHeight = 132;
	const barAreaTop = 16;
	const barAreaHeight = 88;
	const baselineY = 104;
	const barWidth = VOLUME_BAR_WIDTH;

	let svgEl = $state<SVGSVGElement | null>(null);
	let dragging = $state(false);
	let lastIndex = $state<number | null>(null);

	const maxBytes = $derived(Math.max(...volume.map((b) => b.bytes), 1));
	const barCount = $derived(volume.length);

	const bars = $derived(
		volume.map((b, i) => {
			const h = Math.max(4, Math.round((b.bytes / maxBytes) * barAreaHeight));
			const x = volumeBarX(i, barCount);
			return {
				x,
				y: baselineY - h,
				w: barWidth,
				h,
				beforeCutoff: volumeBarCenterX(i, barCount) <= cutoffX
			};
		})
	);

	const yearStart = $derived(volume[0]?.period.slice(0, 4) ?? '');
	const yearEnd = $derived(volume[volume.length - 1]?.period.slice(0, 4) ?? '');

	const cutoffLine = $derived(`M${cutoffX} 10 V112`);
	const shadedWidth = $derived(Math.max(0, cutoffX));
	const showCutoff = $derived(!!cutoffLabel || volume.length > 0);

	function barCenterX(index: number): number {
		return volumeBarCenterX(index, barCount);
	}

	function indexAtX(x: number): number {
		if (!volume.length) return 0;
		let best = 0;
		let bestDist = Infinity;
		for (let i = 0; i < volume.length; i++) {
			const dist = Math.abs(barCenterX(i) - x);
			if (dist < bestDist) {
				bestDist = dist;
				best = i;
			}
		}
		return best;
	}

	function clientToSvgX(clientX: number): number {
		if (!svgEl) return cutoffX;
		const rect = svgEl.getBoundingClientRect();
		return ((clientX - rect.left) / rect.width) * chartWidth;
	}

	function applyIndex(index: number) {
		if (!volume.length) return;
		const idx = Math.max(0, Math.min(volume.length - 1, index));
		cutoffX = barCenterX(idx);
		if (lastIndex !== idx) {
			lastIndex = idx;
			oncutoff?.(idx);
		}
	}

	function onPointerDown(e: PointerEvent) {
		if (!oncutoff || !volume.length || e.button !== 0) return;
		svgEl?.setPointerCapture(e.pointerId);
		dragging = true;
		applyIndex(indexAtX(clientToSvgX(e.clientX)));
	}

	function onPointerMove(e: PointerEvent) {
		if (!dragging) return;
		applyIndex(indexAtX(clientToSvgX(e.clientX)));
	}

	function onPointerUp(e: PointerEvent) {
		if (!dragging) return;
		if (svgEl?.hasPointerCapture(e.pointerId)) svgEl.releasePointerCapture(e.pointerId);
		dragging = false;
	}
</script>

<div class="chart {className}" class:dragging>
	<svg
		bind:this={svgEl}
		viewBox="0 0 {chartWidth} {chartHeight}"
		aria-hidden={!oncutoff}
		role={oncutoff ? 'slider' : undefined}
		aria-valuemin={oncutoff ? 0 : undefined}
		aria-valuemax={oncutoff ? Math.max(volume.length - 1, 0) : undefined}
		aria-valuenow={oncutoff && volume.length ? lastIndex ?? indexAtX(cutoffX) : undefined}
		aria-valuetext={oncutoff && cutoffLabel ? cutoffLabel : undefined}
		aria-label={oncutoff ? 'Отсечка архива' : undefined}
		onpointerdown={onPointerDown}
		onpointermove={onPointerMove}
		onpointerup={onPointerUp}
		onpointercancel={onPointerUp}
	>
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
		{#if showCutoff}
			<path d={cutoffLine} stroke="var(--c)" stroke-width="1.5" fill="none" />
			{#if volume.length}
				<rect
					x={cutoffX - 14}
					y="8"
					width="28"
					height="106"
					fill="transparent"
					class="handle"
				/>
			{/if}
			{#if cutoffLabel}
				<text x={cutoffX} y="126" text-anchor="middle" font-size="10" fill="var(--c)"
					>{cutoffLabel}</text
				>
			{/if}
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
		touch-action: pan-y;
	}
	.chart.dragging svg {
		touch-action: none;
	}
	.handle {
		cursor: ew-resize;
	}
</style>
