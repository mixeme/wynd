import { ApiError, apiJson, isPaymentRequired } from '$lib/api/client';
import {
	getAppSettings,
	saveAppSettings,
	type SessionRecord,
	type Theme,
	getSession,
	listSessions,
	putSession,
	deleteSession,
	deleteCursor,
	invalidateSnapshots,
	getAdminSession,
	putAdminSession,
	clearAdminSession,
	type AdminSessionRecord
} from '$lib/idb/db';

let theme = $state<Theme>('system');
let systemDark = $state(
	typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
);
let initialized = $state(false);

export function getTheme(): Theme {
	return theme;
}

export function isDark(): boolean {
	if (theme === 'dark') return true;
	if (theme === 'light') return false;
	return systemDark;
}

export function setTheme(next: Theme): void {
	theme = next;
	void persistTheme(next);
}

export function initTheme(): () => void {
	if (typeof window === 'undefined') return () => {};
	const mq = window.matchMedia('(prefers-color-scheme: dark)');
	systemDark = mq.matches;
	const onChange = () => {
		systemDark = mq.matches;
	};
	mq.addEventListener('change', onChange);
	return () => mq.removeEventListener('change', onChange);
}

async function persistTheme(next: Theme): Promise<void> {
	const current = (await getAppSettings()) ?? { theme: 'system' as Theme };
	await saveAppSettings({ ...current, theme: next });
}

/** Server rejected the stored Bearer token (missing, expired, or revoked). */
export function isSessionRejected(err: unknown): boolean {
	return (
		err instanceof ApiError &&
		(err.status === 401 || (err.status === 403 && !isPaymentRequired(err)))
	);
}

/** Drop local participant state for an origin after the server no longer accepts the token. */
export async function dropParticipantSession(origin: string): Promise<void> {
	try {
		await apiJson(origin, '/auth/logout', { method: 'POST' });
	} catch {
		/* token already invalid or server unreachable */
	}
	await deleteSession(origin);
	await deleteCursor(origin);
	await invalidateSnapshots(origin);
}

/** Ping the server for each stored session; purge tokens the server no longer knows. */
export async function reconcileStoredSessions(): Promise<void> {
	const sessions = await listSessions();
	for (const session of sessions) {
		try {
			await apiJson(session.origin, '/circles');
		} catch (err) {
			if (isSessionRejected(err)) {
				await dropParticipantSession(session.origin);
			}
		}
	}
}

/** Validate the admin token against the server; drop it locally when rejected. */
export async function reconcileAdminSession(): Promise<AdminSessionRecord | undefined> {
	const session = await getAdminSession();
	if (!session) return undefined;
	try {
		await apiJson('', '/admin/access');
		return session;
	} catch (err) {
		if (isSessionRejected(err)) {
			await clearAdminSession();
			return undefined;
		}
		return session;
	}
}

export async function initSession(): Promise<void> {
	if (initialized) return;
	const settings = await getAppSettings();
	if (settings?.theme) theme = settings.theme;
	await reconcileStoredSessions();
	initialized = true;
}

export async function loadSessions(): Promise<SessionRecord[]> {
	return listSessions();
}

export async function loadSession(origin: string): Promise<SessionRecord | undefined> {
	return getSession(origin);
}

export async function storeSession(session: SessionRecord): Promise<void> {
	await putSession(session);
}

export async function removeSession(origin: string): Promise<void> {
	await dropParticipantSession(origin);
}

export async function loadAdminSession(): Promise<AdminSessionRecord | undefined> {
	return getAdminSession();
}

export async function storeAdminSession(session: AdminSessionRecord): Promise<void> {
	await putAdminSession(session);
}

/**
 * Выход из панели: сначала отзыв токена на сервере, потом чистка локально.
 * Без первого шага «выход» оставлял серверный токен живым на 30 дней (AUTH-3).
 * Сбой сети не должен запирать в панели — локальную запись убираем всё равно.
 */
export async function removeAdminSession(): Promise<void> {
	try {
		await apiJson('', '/admin/logout', { method: 'POST' });
	} catch {
		// токен мог уже истечь или сервер недоступен
	}
	await clearAdminSession();
}
