import type { CircleColor } from '$lib/theme/colors';

/**
 * Строка улочки, с которой открыли круг: первый кадр шапки без чтения базы
 * (C97). Снимок живёт до первого чтения и стареет за десять секунд: после
 * перезапуска приложения он уже не про этот переход.
 */
export interface CircleSeed {
	origin: string;
	id: string;
	name: string;
	color: CircleColor;
	/** Курсор прочитанного: граница «нового» до ответа сети. */
	lastReadSeq: number;
	/** «Читает, не пишет» — ввод не показываем до ответа сети. */
	canWrite: boolean;
	at: number;
}

const SEED_TTL_MS = 10_000;
let seed: CircleSeed | undefined;

export function rememberCircleSeed(next: Omit<CircleSeed, 'at'>): void {
	seed = { ...next, at: Date.now() };
}

/** Снимок для этого круга, если он ещё свежий; забирается один раз. */
export function takeCircleSeed(circleId: string): CircleSeed | undefined {
	if (!seed || seed.id !== circleId || Date.now() - seed.at > SEED_TTL_MS) return undefined;
	const found = seed;
	seed = undefined;
	return found;
}
