import { apiJson } from '$lib/api/client';
import { invalidateSnapshots } from '$lib/idb/db';
import type { ReviveRoom } from '$lib/archive/revive';

/** Перенос круга на новом сервере (план 53): ссылка, карточка, ход, итог. */
export interface IncomingMove {
	id: string;
	status: 'draft' | 'pulling' | 'reviving' | 'done' | 'failed';
	source_url: string;
	source_name: string;
	expires_at: string;
	link_alive: boolean;
	link?: string;
	send_invites: boolean;
	pulled_bytes: number;
	frozen_at?: string;
	/** Код причины сбоя: move_expired, move_taken, archive_invalid, no_room… */
	error?: string;
	circle: { name: string; color: string; posts: number; members: number; bytes: number };
	result?: {
		circle_id: string;
		posts: number;
		names: number;
		bytes: number;
		verified: boolean;
		invited: number;
		not_joined: number;
	};
}

export type IncomingMoveView = IncomingMove & ReviveRoom;

/** Последний перенос этой учётки и место для него; move: null — не было. */
export async function fetchIncomingMove(
	origin: string
): Promise<{ move: IncomingMoveView | null } & Partial<ReviveRoom>> {
	return apiJson(origin, '/moves/incoming');
}

/** Вставили ссылку (2.18): сервер спрашивает карточку у прежнего. */
export function putIncomingMove(origin: string, link: string): Promise<IncomingMoveView> {
	return apiJson<IncomingMoveView>(origin, '/moves/incoming', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ link })
	});
}

/** «Поднять круг здесь» (2.18): дальше сервер работает сам. */
export async function startIncomingMove(
	origin: string,
	id: string,
	sendInvites: boolean
): Promise<IncomingMove> {
	const move = await apiJson<IncomingMove>(origin, `/moves/incoming/${id}/start`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ send_invites: sendInvites })
	});
	await invalidateSnapshots(origin, { kind: 'circles' });
	return move;
}

/** Что сказать по коду ошибки ссылки или сбоя переноса. */
export function moveErrorText(code: string | undefined): string {
	switch (code) {
		case 'link_invalid':
			return 'Это не ссылка переноса Wynd. Она выглядит так: https://адрес-сервера/move/…';
		case 'move_not_found':
			return 'Ссылка не подходит: на том сервере такого переноса нет. Проверьте, что скопировали её целиком.';
		case 'move_canceled':
			return 'Перенос по этой ссылке отменён на прежнем сервере.';
		case 'move_completed':
			return 'Этот круг уже переехал.';
		case 'move_taken':
			return 'Эту ссылку уже взял другой сервер. Ссылка одноразовая: на прежнем сервере отмените перенос и начните его заново.';
		case 'move_expired':
			return 'Срок ссылки вышел. На прежнем сервере отмените перенос и начните его заново — там дадут новую ссылку.';
		case 'source_too_old':
			return 'Сервер по ссылке не умеет переносить круги — его нужно обновить.';
		case 'source_unreachable':
			return 'Сервер по ссылке не отвечает. Проверьте адрес или попробуйте позже.';
		case 'move_busy':
			return 'Этот сервер уже забирает другой ваш круг — дождитесь, пока он закончит.';
		case 'no_room':
			return 'Круг сюда не поместился.';
		case 'archive_invalid':
		case 'archive_changed':
			return 'Файл круга пришёл с прежнего сервера повреждённым.';
	}
	return 'Перенос не удался.';
}
