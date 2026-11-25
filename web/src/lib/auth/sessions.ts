import { apiJson } from '$lib/api/client';

// Сессии устройств своей учётной записи (план 72). Метка устройства и
// времена отдаёт сервер; IP и сырого User-Agent в ответе нет.

export interface SessionItem {
	/** Открытый id сессии (префикс хеша токена) — им же и отзывают. */
	id: string;
	device: string;
	created_at: string;
	last_seen_at: string;
	expires_at: string;
	/** Эта сессия — устройство, с которого пришёл запрос. */
	current: boolean;
}

export async function fetchSessions(origin: string): Promise<SessionItem[]> {
	const data = await apiJson<{ sessions: SessionItem[] }>(origin, '/sessions');
	return data.sessions;
}

export async function revokeSession(origin: string, id: string): Promise<void> {
	await apiJson(origin, `/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
