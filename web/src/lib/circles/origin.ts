import { normalizeOrigin } from '$lib/api/client';
import { fetchCircles, fetchPendingCircleJoins, loadCirclesCached } from '$lib/circles/circles';
import { listSessions } from '$lib/idb/db';

const LAST_CIRCLE_KEY = 'wynd:last-circle';

export function rememberLastCircle(circleId: string): void {
	if (typeof sessionStorage === 'undefined') return;
	sessionStorage.setItem(LAST_CIRCLE_KEY, circleId);
}

export function peekLastCircle(): string | null {
	if (typeof sessionStorage === 'undefined') return null;
	return sessionStorage.getItem(LAST_CIRCLE_KEY);
}

function storageKey(circleId: string): string {
	return `wynd:circle:${circleId}:origin`;
}

export function rememberCircleOrigin(circleId: string, origin: string): void {
	if (typeof sessionStorage === 'undefined') return;
	sessionStorage.setItem(storageKey(circleId), normalizeOrigin(origin));
}

export function peekCircleOrigin(circleId: string): string | null {
	if (typeof sessionStorage === 'undefined') return null;
	const stored = sessionStorage.getItem(storageKey(circleId));
	return stored !== null ? stored : null;
}

export async function resolveCircleOrigin(circleId: string): Promise<string | null> {
	const stored = peekCircleOrigin(circleId);
	if (stored !== null) return stored;

	const sessions = await listSessions();
	// Сперва снимки (C104): определение сервера круга не должно ждать сеть,
	// иначе прямой адрес круга (в том числе «Этот круг» из общего поиска)
	// держит заставку до ответа сервера. Сеть ниже всё равно сверит список
	// и поправит устаревший снимок.
	for (const session of sessions) {
		const cached = await loadCirclesCached(session.origin);
		if (cached.some((c) => c.id === circleId)) {
			rememberCircleOrigin(circleId, session.origin);
			return normalizeOrigin(session.origin);
		}
	}
	for (const session of sessions) {
		let circles;
		try {
			circles = await fetchCircles(session.origin);
		} catch {
			circles = await loadCirclesCached(session.origin);
		}
		if (circles.some((c) => c.id === circleId)) {
			rememberCircleOrigin(circleId, session.origin);
			return normalizeOrigin(session.origin);
		}
	}
	for (const session of sessions) {
		try {
			const pending = await fetchPendingCircleJoins(session.origin);
			if (pending.includes(circleId)) {
				rememberCircleOrigin(circleId, session.origin);
				return normalizeOrigin(session.origin);
			}
		} catch {
			/* origin unreachable */
		}
	}
	return null;
}
