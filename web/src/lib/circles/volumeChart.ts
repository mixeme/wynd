/** Shared geometry for VolumeChart and quota cutoff sync. */
export const VOLUME_CHART_WIDTH = 358;
export const VOLUME_BAR_WIDTH = 18;
export const VOLUME_BAR_GAP = 8;
export const VOLUME_BAR_STEP = VOLUME_BAR_WIDTH + VOLUME_BAR_GAP;

/** Natural 26px step, or compressed so the pack fits the viewBox. */
export function volumeBarStep(barCount: number): number {
	if (barCount <= 1) return VOLUME_BAR_STEP;
	const packed = (barCount - 1) * VOLUME_BAR_STEP + VOLUME_BAR_WIDTH;
	if (packed <= VOLUME_CHART_WIDTH) return VOLUME_BAR_STEP;
	return (VOLUME_CHART_WIDTH - VOLUME_BAR_WIDTH) / (barCount - 1);
}

export function volumeChartOffsetX(barCount: number): number {
	if (!barCount) return 0;
	const contentWidth = (barCount - 1) * volumeBarStep(barCount) + VOLUME_BAR_WIDTH;
	return Math.max(0, (VOLUME_CHART_WIDTH - contentWidth) / 2);
}

export function volumeBarX(index: number, barCount: number): number {
	return volumeChartOffsetX(barCount) + index * volumeBarStep(barCount);
}

export function volumeBarCenterX(index: number, barCount: number): number {
	return volumeBarX(index, barCount) + VOLUME_BAR_WIDTH / 2;
}

/** Левый край столбика — середина зазора перед ним. Здесь линия отсечки,
 *  когда дата — начало столбика: «всё до 1 марта» сам март не берёт. */
export function volumeBarEdgeX(index: number, barCount: number): number {
	const gap = Math.max(0, volumeBarStep(barCount) - VOLUME_BAR_WIDTH);
	return volumeBarX(index, barCount) - Math.min(gap, VOLUME_BAR_GAP) / 2;
}
