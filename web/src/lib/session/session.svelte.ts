import { ApiError, apiJson, isPaymentRequired } from '$lib/api/client';
import { unsubscribePush } from '$lib/push/push';
import {
	getAppSettings,
	saveAppSettings,
	type AvatarShape,
	type SessionRecord,
	type Theme,
	getSession,
	listSessions,
	putSession,
	deleteSession,
	deleteCursor,
	invalidateSnapshots,
	clearOriginState,
	getAdminSession,
	putAdminSession,
	clearAdminSession,
	type AdminSessionRecord
} from '$lib/idb/db';

let theme = $state<Theme>('system');
// Форма аватаров (A16): одна на все устройства — хранится у учётной записи на
// сервере, здесь — для мгновенного показа при запуске.
let avatarShape = $state<AvatarShape>('circle');
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

export function getAvatarShape(): AvatarShape {
	return avatarShape;
}

/** Выбор в «Настройки → Приложение»: здесь и на всех серверах, где вошли. */
export async function setAvatarShape(next: AvatarShape): Promise<void> {
	avatarShape = next;
	const current = (await getAppSettings()) ?? { theme: 'system' as Theme };
	await saveAppSettings({ ...current, avatar_shape: next });
	const sessions = await listSessions();
	await Promise.all(
		sessions.map((s) =>
			apiJson(s.origin, '/appearance', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ avatar_shape: next })
			}).catch(() => undefined)
		)
	);
}

/**
 * Форма с сервера: её могли сменить на другом устройстве. Берём ответ
 * первого сервера, который ответил; сервер до 0.43 формы не знает — остаётся
 * своя.
 */
async function syncAvatarShape(): Promise<void> {
	for (const s of await listSessions()) {
		try {
			const res = await apiJson<{ avatar_shape?: string }>(s.origin, '/appearance');
			const shape: AvatarShape = res.avatar_shape === 'square' ? 'square' : 'circle';
			if (shape !== avatarShape) {
				avatarShape = shape;
				const current = (await getAppSettings()) ?? { theme: 'system' as Theme };
				await saveAppSettings({ ...current, avatar_shape: shape });
			}
			return;
		} catch {
			/* следующий сервер */
		}
	}
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

/**
 * Drop local participant state for an origin after the server no longer
 * accepts the token — or the user signed out. Push-подписка снимается до
 * отзыва токена (нужна сессия), очередь и кэш медиа стираются вместе со
 * снимками: устройство могло сменить хозяина (аудит 2026-09-22).
 */
export async function dropParticipantSession(origin: string): Promise<void> {
	try {
		await unsubscribePush(origin);
	} catch {
		/* no push on this device or token already invalid */
	}
	try {
		await apiJson(origin, '/auth/logout', { method: 'POST' });
	} catch {
		/* token already invalid or server unreachable */
	}
	await deleteSession(origin);
	await deleteCursor(origin);
	await invalidateSnapshots(origin);
	await clearOriginState(origin);
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
	if (settings?.avatar_shape) avatarShape = settings.avatar_shape;
	await reconcileStoredSessions();
	initialized = true;
	void syncAvatarShape();
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
