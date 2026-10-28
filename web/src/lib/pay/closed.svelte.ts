// Серверы, где подписка закончилась (C67, кадры 10.16 и 10.17). У человека
// круги могут жить на нескольких серверах: шлюз спрашивает каждый, и закрытый
// сервер закрывает только свои круги — они стоят на улочке приглушёнными, а
// нажатие ведёт на «Доступ закрыт» этого сервера. Сервер один — шлюз, как 10.1.
import { normalizeOrigin } from '$lib/api/client';
import type { SessionRecord } from '$lib/idb/db';

let closed = $state<string[]>([]);

export function setClosedServers(origins: string[]): void {
	closed = origins.map(normalizeOrigin);
}

export function isServerClosed(origin: string): boolean {
	return closed.includes(normalizeOrigin(origin));
}

/** Значение `?closed=` / `?server=`: свой сервер — '', в адресе — его origin. */
export function serverParam(origin: string): string {
	return encodeURIComponent(normalizeOrigin(origin) || window.location.origin);
}

/** Сессия сервера из адреса; без параметра — первая, как было до C67. */
export function sessionForParam(
	sessions: SessionRecord[],
	value: string | null
): SessionRecord | undefined {
	if (value === null) return sessions[0];
	const want = normalizeOrigin(value);
	return sessions.find((s) => normalizeOrigin(s.origin) === want);
}
