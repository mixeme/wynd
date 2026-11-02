import { fetchInstance, type InstanceInfo } from '$lib/auth/auth';
import { getSnapshot, putSnapshot, snapshotKey } from '$lib/idb/db';

export type ServerConnection = 'checking' | 'unavailable';
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
	} catch {
		return { info: saved, connection: 'unavailable' };
	}
}
