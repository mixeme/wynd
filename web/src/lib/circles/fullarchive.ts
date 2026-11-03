import { apiJson, apiPath } from '$lib/api/client';
import { invalidateSnapshots } from '$lib/idb/db';
import { openDownloadAfter } from '$lib/navigation/openDownload';
import type { ArchiveKey } from '$lib/archive/fullkey';

/** Полный архив круга (план 51): что с ним на сервере. */
export interface FullArchive {
	full_journal?: boolean;
	posts?: number;
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
	full_journal?: boolean;
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
export async function removeFrozenCircle(
	origin: string,
	circleId: string,
	acknowledgePartial = false
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/full_archive/remove_circle`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ acknowledge_partial: acknowledgePartial })
	});
	await invalidateSnapshots(origin, { kind: 'circles' });
}

export const PARTIAL_ARCHIVE_WARNING =
	'В файле сохранена только часть журнала. При удалении круга с сервера будут удалены и записи, которые не вошли в файл. Восстановить их из этого файла не получится.';

/** Скачать готовый файл: качает сам браузер, с ходом и докачкой. */
export async function downloadFullArchive(origin: string, circleId: string): Promise<void> {
	await openDownloadAfter(async () => {
		const { url } = await apiJson<{ url: string }>(
			origin,
			`/circles/${circleId}/full_archive/link`,
			{
				method: 'POST'
			}
		);
		return apiPath(origin, url.replace(/^\/api\/v1/, ''));
	});
}

/** «Скачать сразу» (6.31): сервер собирает файл прямо в скачивание. */
export async function streamFullArchive(
	origin: string,
	circleId: string,
	key: ArchiveKey | undefined | (() => Promise<ArchiveKey | undefined>),
	deleteCircle = false
): Promise<void> {
	await openDownloadAfter(async () => {
		const archiveKey = typeof key === 'function' ? await key() : key;
		const { url } = await apiJson<{ url: string }>(
			origin,
			`/circles/${circleId}/full_archive/stream_link`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ key: archiveKey, delete_circle: deleteCircle })
			}
		);
		return apiPath(origin, url.replace(/^\/api\/v1/, ''));
	});
}
