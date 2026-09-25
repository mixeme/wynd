import {
	getAppSettings,
	saveAppSettings,
	type SessionRecord,
	type Theme,
	getSession,
	listSessions,
	putSession,
	deleteSession,
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

export async function initSession(): Promise<void> {
	if (initialized) return;
	const settings = await getAppSettings();
	if (settings?.theme) theme = settings.theme;
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
	await deleteSession(origin);
}

export async function loadAdminSession(): Promise<AdminSessionRecord | undefined> {
	return getAdminSession();
}

export async function storeAdminSession(session: AdminSessionRecord): Promise<void> {
	await putAdminSession(session);
}

export async function removeAdminSession(): Promise<void> {
	await clearAdminSession();
}
