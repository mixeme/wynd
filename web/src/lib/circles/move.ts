import { apiJson } from '$lib/api/client';
import { invalidateSnapshots } from '$lib/idb/db';

/** Перенос круга на прежнем сервере (план 53): что с ним сейчас. */
export interface CircleMove {
	id: string;
	status: 'waiting' | 'pulling' | 'accepted';
	started_at: string;
	expires_at: string;
	/** Ссылка жива: по ней ещё можно забрать круг (72 часа, 6.25). */
	link_alive: boolean;
	/** Сама ссылка — только владельцу и пока она жива. */
	link?: string;
	archive_status: 'none' | 'building' | 'ready' | 'failed';
	archive_bytes: number;
	pulled_bytes: number;
	/** Новый сервер — из обмена, когда он пришёл по ссылке. */
	dest_url?: string;
	dest_name?: string;
	accepted_at?: string;
	accepted_posts?: number;
	accepted_names?: number;
	accepted_bytes?: number;
}

export interface CircleMoveState {
	/** Что уедет (6.24). */
	estimate: { posts: number; members: number; bytes: number };
	move: CircleMove | null;
	frozen_at?: string;
}

export function fetchCircleMove(origin: string, circleId: string): Promise<CircleMoveState> {
	return apiJson<CircleMoveState>(origin, `/circles/${circleId}/move`);
}

/** «Остановить круг и получить ссылку» (6.24). */
export async function startCircleMove(origin: string, circleId: string): Promise<CircleMove> {
	const move = await apiJson<CircleMove>(origin, `/circles/${circleId}/move`, { method: 'POST' });
	await invalidateSnapshots(origin, { kind: 'circles' });
	return move;
}

/** «Отменить перенос» (6.25, 6.26): ссылка гаснет, круг оживает. */
export async function cancelCircleMove(origin: string, circleId: string): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/move/cancel`, { method: 'POST' });
	await invalidateSnapshots(origin, { kind: 'circles' });
}

/** «Завершить перенос» (6.26): круг удаляется здесь, участникам — письмо. */
export async function completeCircleMove(origin: string, circleId: string): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/move/complete`, { method: 'POST' });
	await invalidateSnapshots(origin, { kind: 'circles' });
}

/** Адрес сервера без «https://» — как его пишут на кадрах: «wynd.u-slavy.ru». */
export function serverHost(url: string | undefined): string {
	if (!url) return '';
	try {
		return new URL(url).host;
	} catch {
		return url.replace(/^https?:\/\//, '').replace(/\/+$/, '');
	}
}
