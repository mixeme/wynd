/**
 * Свайп влево-вправо по вкладкам круга (план 46, C9).
 *
 * Палец ведёт содержимое; отпустил дальше трети ширины — соседняя вкладка,
 * иначе возврат. Жест горизонтальный, только если ход вбок заметно больше
 * хода вниз: вертикальная прокрутка и «потянуть, чтобы обновить» не задеты.
 * На «Карте» карта двигается сама — там жест только от края экрана.
 */
export const SWIPE = {
	/** До этого хода палец просто касается. */
	slop: 10,
	/** Вбок должно быть во столько раз больше, чем вниз. */
	dominance: 1.3,
	/** Доля ширины, после которой отпускание переключает вкладку. */
	commit: 1 / 3,
	/** Полоса у края, откуда на «Карте» можно начать жест. */
	edge: 24
} as const;

export type SwipePhase = 'idle' | 'pending' | 'horizontal' | 'ignored';

export interface SwipeState {
	phase: SwipePhase;
	startX: number;
	startY: number;
	dx: number;
	width: number;
}

export function swipeIdle(): SwipeState {
	return { phase: 'idle', startX: 0, startY: 0, dx: 0, width: 0 };
}

/** Касание. edgeOnly — жест только от края (вкладка «Карта»). */
export function swipeStart(x: number, y: number, width: number, edgeOnly: boolean): SwipeState {
	if (edgeOnly && x > SWIPE.edge && x < width - SWIPE.edge) {
		return { ...swipeIdle(), phase: 'ignored' };
	}
	return { phase: 'pending', startX: x, startY: y, dx: 0, width };
}

/** Ход пальца: решаем направление один раз, дальше ведём по горизонтали. */
export function swipeMove(state: SwipeState, x: number, y: number): SwipeState {
	if (state.phase === 'idle' || state.phase === 'ignored') return state;
	const dx = x - state.startX;
	const dy = y - state.startY;
	if (state.phase === 'pending') {
		if (Math.abs(dx) < SWIPE.slop && Math.abs(dy) < SWIPE.slop) return state;
		if (Math.abs(dx) < Math.abs(dy) * SWIPE.dominance) return { ...state, phase: 'ignored', dx: 0 };
	}
	return { ...state, phase: 'horizontal', dx };
}

/**
 * Отпустил. -1 — на вкладку левее (палец вправо), 1 — правее (палец влево),
 * 0 — остаёмся. hasPrev/hasNext — есть ли соседняя вкладка.
 */
export function swipeEnd(state: SwipeState, hasPrev: boolean, hasNext: boolean): -1 | 0 | 1 {
	if (state.phase !== 'horizontal' || state.width <= 0) return 0;
	if (Math.abs(state.dx) < state.width * SWIPE.commit) return 0;
	if (state.dx < 0 && hasNext) return 1;
	if (state.dx > 0 && hasPrev) return -1;
	return 0;
}

/** Сдвиг содержимого под пальцем: у крайней вкладки — с сопротивлением. */
export function swipeOffset(state: SwipeState, hasPrev: boolean, hasNext: boolean): number {
	if (state.phase !== 'horizontal') return 0;
	const blocked = (state.dx > 0 && !hasPrev) || (state.dx < 0 && !hasNext);
	return blocked ? state.dx / 4 : state.dx;
}
