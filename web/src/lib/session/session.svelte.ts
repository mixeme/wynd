import { nativeClear } from '$lib/android/native-queue';
import { androidSecretsAvailable } from '$lib/android/secrets';
import { ApiError, apiJson, isPaymentRequired, normalizeOrigin } from '$lib/api/client';
import { forgetStreetMemory } from '$lib/circles/street-memory';
import { forgetSearchScreens } from '$lib/journal/search-memory';
import { refreshAndroidPush, unsubscribePush } from '$lib/push/push';
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
	migrateSessionsToSystemStorage,
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
// Одновременные вызовы initSession (макет и экран) делят одну работу.
let initFlight: Promise<void> | undefined;

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
	// Нативная очередь Android (план 50, этап 5.1): после выхода её токен
	// больше не должен отправлять записи этого сервера.
	try {
		await nativeClear(
			normalizeOrigin(origin) || (typeof window !== 'undefined' ? window.location.origin : '')
		);
	} catch {
		/* Старая оболочка без моста очереди. */
	}
	await deleteCursor(origin);
	await invalidateSnapshots(origin);
	await clearOriginState(origin);
	// Улочка в памяти захода — тоже состояние входа (C97): новый человек не
	// должен увидеть чужие названия даже одним кадром.
	forgetStreetMemory();
	// Строки поиска тоже помнят названия и отрывки (C160).
	forgetSearchScreens();
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

/**
 * Локальная готовность приложения: настройки с устройства. Проверка входов
 * идёт фоном (C99): запуск не ждёт сети, а отозванный вход исчезает, когда
 * сервер ответит. Снимки экранов к этому моменту уже на устройстве.
 */
export function initSession(): Promise<void> {
	if (initialized) return Promise.resolve();
	if (!initFlight) {
		initFlight = doInitSession().catch((err) => {
			initFlight = undefined;
			throw err;
		});
	}
	return initFlight;
}

async function doInitSession(): Promise<void> {
	const settings = await getAppSettings();
	if (settings?.theme) theme = settings.theme;
	if (settings?.avatar_shape) avatarShape = settings.avatar_shape;
	// Токены переезжают в учётные записи Windows при первом запуске новой
	// оболочки (план 66, функция 3); профиль очищается от них после выгрузки.
	// В приложении Android (C127) входы переезжают в общий файл оболочки до
	// первого чтения — иначе улица на миг осталась бы без входов.
	if (androidSecretsAvailable()) await migrateSessionsToSystemStorage().catch(() => {});
	else void migrateSessionsToSystemStorage().catch(() => {});
	initialized = true;
	// Не ждём: экран уже может показывать снимок, сервер ответит следом.
	void syncAvatarShape();
	void reconcileStoredSessions().catch(() => {});
}

export async function loadSessions(): Promise<SessionRecord[]> {
	return listSessions();
}

export async function loadSession(origin: string): Promise<SessionRecord | undefined> {
	return getSession(origin);
}

export async function storeSession(session: SessionRecord): Promise<void> {
	await putSession(session);
	refreshAndroidPush();
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
