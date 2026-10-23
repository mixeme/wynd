import { apiJson, apiPath } from '$lib/api/client';
import { invalidateSnapshots } from '$lib/idb/db';
import { openDownload } from '$lib/navigation/openDownload';
import type { ArchiveKey } from '$lib/archive/fullkey';

/** Полный архив круга (план 51): что с ним на сервере. */
export interface FullArchive {
	id: string;
	status: 'building' | 'ready' | 'failed' | 'removed';
	requested_at: string;
	snapshot_at?: string;
	ready_at?: string;
	expires_at?: string;
	downloaded_at?: string;
	removed_at?: string;
	removed_reason?: 'downloaded' | 'expired' | 'admin' | 'owner' | 'canceled';
	delete_circle: boolean;
	encrypted: boolean;
	size_bytes: number;
}

export interface FullArchiveState {
	/** Круг замер под архив с удалением (6.25): с этого момента. */
	frozen_at?: string | null;
	estimate_bytes: number;
	posts: number;
	members: number;
	/** Готовый файл помещается на сервер; нет — 6.31, «Скачать сразу». */
	fits: boolean;
	retention_hours: number;
	archive: FullArchive | null;
}

export function fetchFullArchive(origin: string, circleId: string): Promise<FullArchiveState> {
	return apiJson<FullArchiveState>(origin, `/circles/${circleId}/full_archive`);
}

/** «Собрать архив» (6.24, 6.25). Ключ — открытый ключ и соль, пароль остаётся тут. */
export function startFullArchive(
	origin: string,
	circleId: string,
	key: ArchiveKey | undefined,
	deleteCircle = false
): Promise<FullArchive> {
	return apiJson<FullArchive>(origin, `/circles/${circleId}/full_archive`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ key, delete_circle: deleteCircle })
	});
}

/** «Отменить сборку» (6.26) и «Удалить архив, не скачивая» (6.27). */
export async function removeFullArchive(origin: string, circleId: string): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/full_archive`, { method: 'DELETE' });
}

/** «Отменить — оживить круг» (6.28, 6.29, 6.30). */
export async function reviveCircle(origin: string, circleId: string): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/full_archive/revive`, { method: 'POST' });
	await invalidateSnapshots(origin, { kind: 'circles' });
}

/** «Убрать круг с сервера» (6.29): только когда файл скачан. */
export async function removeFrozenCircle(origin: string, circleId: string): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/full_archive/remove_circle`, { method: 'POST' });
	await invalidateSnapshots(origin, { kind: 'circles' });
}

function openServerDownload(origin: string, url: string) {
	openDownload(apiPath(origin, url.replace(/^\/api\/v1/, '')));
}

/** Скачать готовый файл: качает сам браузер, с ходом и докачкой. */
export async function downloadFullArchive(origin: string, circleId: string): Promise<void> {
	const { url } = await apiJson<{ url: string }>(origin, `/circles/${circleId}/full_archive/link`, {
		method: 'POST'
	});
	openServerDownload(origin, url);
}

/** «Скачать сразу» (6.31): сервер собирает файл прямо в скачивание. */
export async function streamFullArchive(
	origin: string,
	circleId: string,
	key: ArchiveKey | undefined,
	deleteCircle = false
): Promise<void> {
	const { url } = await apiJson<{ url: string }>(
		origin,
		`/circles/${circleId}/full_archive/stream_link`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ key, delete_circle: deleteCircle })
		}
	);
	openServerDownload(origin, url);
}
