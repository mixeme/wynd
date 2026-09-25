/**
 * Автомат «потянуть, чтобы обновить».
 *
 * Жил в экране ленты: состояние жеста хранилось в `feedEl.dataset`, пороги
 * были магическими числами, а `setTimeout` при уходе с экрана не снимался
 * (GUI-5). Здесь та же логика без DOM — её видно и можно проверить тестом.
 *
 * Правила: тянем только от самого верха списка; ход ограничен `maxPull`;
 * отпустили дальше `threshold` — полоса замирает на `restHeight`, и через
 * `settleMs` начинается обновление. Всё остальное просто возвращает полосу.
 */
export const PTR = {
	/** Дальше этого палец не тянет полосу. */
	maxPull: 80,
	/** За этой чертой отпускание запускает обновление. */
	threshold: 48,
	/** Высота полосы, пока идёт обновление. */
	restHeight: 69,
	/** Пауза между «отпустил» и запросом — на ней доигрывает знак. */
	settleMs: 280
} as const;

export type PullPhase = 'idle' | 'pulling' | 'settling' | 'refreshing';

export interface PullState {
	phase: PullPhase;
	/** Насколько полоса вытянута сейчас. */
	pull: number;
	/** Точка, с которой начался жест; null — жест не начат. */
	startY: number | null;
}

export function pullIdle(): PullState {
	return { phase: 'idle', pull: 0, startY: null };
}

/** Палец коснулся списка. Жест начинается только от самого верха. */
export function pullStart(state: PullState, y: number, scrollTop: number): PullState {
	if (state.phase === 'settling' || state.phase === 'refreshing') return state;
	if (scrollTop > 0) return { ...state, startY: null, pull: 0, phase: 'idle' };
	return { phase: 'pulling', pull: 0, startY: y };
}

/** Палец ведёт вниз. Вверх не тянем, дальше maxPull — тоже. */
export function pullMove(state: PullState, y: number, scrollTop: number): PullState {
	if (state.phase !== 'pulling' || state.startY === null) return state;
	if (scrollTop > 0) return { ...state, pull: 0 };
	const pull = Math.max(0, Math.min(PTR.maxPull, y - state.startY));
	return { ...state, pull };
}

/**
 * Палец отпустили. Дальше порога — фаза settling: полоса замирает, а
 * вызывающий через PTR.settleMs зовёт pullSettled. Иначе всё возвращается.
 */
export function pullEnd(state: PullState): PullState {
	if (state.phase !== 'pulling') return state;
	if (state.pull > PTR.threshold) {
		return { phase: 'settling', pull: PTR.restHeight, startY: null };
	}
	return pullIdle();
}

/** Пауза доиграла — пора обновлять. */
export function pullSettled(state: PullState): PullState {
	if (state.phase !== 'settling') return state;
	return { phase: 'refreshing', pull: PTR.restHeight, startY: null };
}

/** Обновление закончилось (успехом или ошибкой — неважно). */
export function pullFinished(): PullState {
	return pullIdle();
}

/** Высота полосы для отрисовки. */
export function pullHeight(state: PullState): number {
	if (state.phase === 'settling' || state.phase === 'refreshing') return PTR.restHeight;
	return state.pull;
}

/** Высота знака внутри полосы: он подрастает вместе с жестом. */
export function pullMarkHeight(state: PullState): number {
	if (state.phase === 'settling' || state.phase === 'refreshing') return PTR.restHeight;
	return Math.max(8, state.pull * 0.85);
}

/** Показывать ли полосу вообще. */
export function pullVisible(state: PullState): boolean {
	return state.phase !== 'idle' || state.pull > 0;
}
