import { normalizeOrigin } from '$lib/api/client';
import { fetchCircles, loadCirclesCached } from '$lib/circles/circles';
import { listSessions } from '$lib/idb/db';

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
	return null;
}
