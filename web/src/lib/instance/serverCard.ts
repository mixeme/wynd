import { fetchInstance, type InstanceInfo } from '$lib/auth/auth';
import { getSnapshot, putSnapshot, snapshotKey } from '$lib/idb/db';

export type ServerConnection = 'checking' | 'unavailable' | 'outdated';
export interface ServerCardState {
	info?: InstanceInfo;
	connection?: ServerConnection;
}

/** Режим регистрации описывает приём новых людей, а не доступность сервера. */
export function registrationCaption(mode: InstanceInfo['registration_mode']): string {
	return {
		open: 'Открыт для новых',
		invite: 'Только по приглашению',
		closed: 'Регистрация закрыта'
	}[mode];
}

/** Сохранённые свойства остаются сведениями сервера; они не доказывают связь. */
export async function loadServerCard(origin: string): Promise<ServerCardState> {
	const key = snapshotKey(origin, 'instance', '');
	let saved: InstanceInfo | undefined;
	try {
		saved = (await getSnapshot(key))?.data as InstanceInfo | undefined;
	} catch {
		// Отсутствие локального хранилища не мешает свежему запросу.
	}
	try {
		const info = await fetchInstance(origin);
		await putSnapshot(key, { data: info, fetched_at: Date.now() }).catch(() => {});
		return { info };
	} catch (err) {
		// Старый сервер без CORS (C123, шаг 5) отвечает сети, но не клиенту с
		// чужого адреса: обычный запрос падает TypeError, а прямой
		// «прозрачный» (no-cors) доходит. Тогда это не «не отвечает», а
		// «устарел» — вход откроется после обновления сервера. HTTP-ошибка
		// (ApiError) и мёртвая сеть остаются «не отвечает».
		if (err instanceof TypeError && (await reachableWithoutCors(origin))) {
			return { info: saved, connection: 'outdated' };
		}
		return { info: saved, connection: 'unavailable' };
	}
}

/** Сервер слышен без CORS? Оpaque-ответ значит «да», отказ — «нет». */
async function reachableWithoutCors(origin: string): Promise<boolean> {
	try {
		await fetch(`${origin}/api/v1/instance`, { mode: 'no-cors', cache: 'no-store' });
		return true;
	} catch {
		return false;
	}
}
