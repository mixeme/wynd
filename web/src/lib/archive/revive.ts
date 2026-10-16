import { apiFetch, apiJson } from '$lib/api/client';
import { invalidateSnapshots } from '$lib/idb/db';
import {
	blobSource,
	decryptedSource,
	inspectWynd,
	openArchive,
	type EncryptedHeader,
	type RandomSource,
	type WyndManifest,
	type WyndTable,
	type ZipReader
} from './wyndfile';

/**
 * Оживление круга из полного архива (план 52) на стороне устройства: файл
 * открывается здесь (пароль не уходит), файлы круга грузятся обычными
 * загрузками частями, потом сервер получает строки таблиц.
 */

export type OpenedFile =
	| { encrypted: false; src: RandomSource }
	| { encrypted: true; src: RandomSource; header: EncryptedHeader; headerLength: number };

export async function openFile(file: File): Promise<OpenedFile> {
	const src = blobSource(file);
	const info = await inspectWynd(src);
	return info.encrypted ? { ...info, src } : { encrypted: false, src };
}

export async function unlock(
	opened: OpenedFile,
	password: string,
	onKeyProgress?: (f: number) => void
): Promise<RandomSource> {
	if (!opened.encrypted) return opened.src;
	return decryptedSource(opened.src, opened.header, opened.headerLength, password, onKeyProgress);
}

export interface ArchiveBlob {
	id: string;
	sha256: string;
	size: number;
	mime: string;
	filename: string;
	/** Запись в ZIP: media/<id>.<расш>. */
	entry: string;
}

export interface LoadedArchive {
	zip: ZipReader;
	manifest: WyndManifest;
	tables: Record<string, WyndTable>;
	blobs: ArchiveBlob[];
	/** Для 7.29: «340 записей · 22 участника · 8,4 ГБ», «Последняя запись — …». */
	posts: number;
	members: number;
	/** Сколько почт позовут письмом: все, кроме того, кто оживляет. */
	emails: string[];
	bytes: number;
	lastEntry: string;
}

function rows(t: WyndTable | undefined): Record<string, unknown>[] {
	if (!t) return [];
	return t.rows.map((r) => Object.fromEntries(t.columns.map((c, i) => [c, r[i]])));
}

export async function loadArchive(src: RandomSource): Promise<LoadedArchive> {
	const { zip, manifest } = await openArchive(src);
	const tables: Record<string, WyndTable> = {};
	for (const name of manifest.tables) {
		tables[name] = await zip.json<WyndTable>(`data/${name}.json`);
	}
	const media = [...zip.entries.keys()].filter((n) => n.startsWith('media/'));
	const blobs: ArchiveBlob[] = rows(tables.blobs).map((b) => {
		const id = String(b.id);
		return {
			id,
			sha256: String(b.sha256),
			size: Number(b.size_bytes),
			mime: String(b.mime_type ?? 'application/octet-stream'),
			filename: String(b.original_filename ?? ''),
			entry: media.find((n) => n.slice('media/'.length).startsWith(id)) ?? ''
		};
	});
	const posts = rows(tables.posts).filter((p) => !Number(p.deleted));
	const lastEntry = posts.reduce((max, p) => (String(p.entry_date) > max ? String(p.entry_date) : max), '');
	const members = rows(tables.memberships).filter((m) => m.status !== 'gone');
	return {
		zip,
		manifest,
		tables,
		blobs,
		posts: posts.length,
		members: members.length,
		emails: rows(tables.accounts).map((a) => String(a.email)),
		bytes: blobs.reduce((n, b) => n + b.size, 0),
		lastEntry
	};
}

export interface ReviveRoom {
	room_bytes: number;
	fits: boolean;
	request?: {
		status: 'pending' | 'approved' | 'rejected';
		requested_bytes: number;
		circle_name: string;
		created_at: string;
		resolved_at?: string;
	};
}

export function fetchReviveRoom(origin: string, bytes: number): Promise<ReviveRoom> {
	return apiJson<ReviveRoom>(origin, `/revive/room?bytes=${Math.max(0, Math.floor(bytes))}`);
}

export async function requestRevivePlace(origin: string, bytes: number, circleName: string): Promise<void> {
	await apiJson(origin, '/revive/place_request', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ bytes, circle_name: circleName })
	});
}

const CHUNK = 1024 * 1024;

async function uploadOne(
	origin: string,
	archive: LoadedArchive,
	blob: ArchiveBlob,
	onBytes: (n: number) => void
): Promise<string> {
	const session = await apiJson<{ id: string }>(origin, '/uploads', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			expected_size: blob.size,
			mime_type: blob.mime,
			filename: blob.filename || undefined
		})
	});
	let offset = 0;
	let failures = 0;
	// Файл кусками из архива, без копии целиком в памяти; обрыв связи —
	// узнать у сервера, сколько он принял, и продолжить оттуда (7.29).
	while (offset < blob.size) {
		let sent = 0;
		try {
			for await (const part of archive.zip.chunks(blob.entry, CHUNK)) {
				const end = sent + part.length;
				if (end <= offset) {
					sent = end;
					continue;
				}
				const piece = part.subarray(offset - sent);
				const res = await apiFetch(origin, `/uploads/${session.id}`, {
					method: 'PUT',
					headers: { 'Content-Type': 'application/octet-stream', 'Upload-Offset': String(offset) },
					body: new Uint8Array(piece)
				});
				const body = (await res.json()) as { received_bytes: number };
				onBytes(body.received_bytes - offset);
				offset = body.received_bytes;
				sent = end;
			}
		} catch (err) {
			failures++;
			if (failures > 8) throw err;
			await new Promise((r) => setTimeout(r, Math.min(30_000, 1000 * 2 ** failures)));
			const head = await apiFetch(origin, `/uploads/${session.id}`, { method: 'HEAD' });
			offset = parseInt(head.headers.get('Upload-Offset') ?? '0', 10);
		}
	}
	const done = await apiJson<{ id: string }>(origin, `/uploads/${session.id}/complete`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ sha256: blob.sha256 })
	});
	return done.id;
}

/** Файлы круга на сервер: уже лежащие у этой учётки — не грузятся. */
export async function uploadArchiveFiles(
	origin: string,
	archive: LoadedArchive,
	onProgress: (sent: number, total: number) => void
): Promise<Record<string, string>> {
	const { blobs: have } = await apiJson<{ blobs: Record<string, string> }>(origin, '/revive/blobs', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ sha256: archive.blobs.map((b) => b.sha256) })
	});
	const map: Record<string, string> = {};
	let sent = 0;
	onProgress(0, archive.bytes);
	for (const blob of archive.blobs) {
		const existing = have[blob.sha256];
		if (existing) {
			map[blob.id] = existing;
			sent += blob.size;
			onProgress(sent, archive.bytes);
			continue;
		}
		map[blob.id] = await uploadOne(origin, archive, blob, (n) => {
			sent += n;
			onProgress(sent, archive.bytes);
		});
	}
	return map;
}

export interface ReviveResult {
	circle_id: string;
	posts: number;
	names: number;
	bytes: number;
	verified: boolean;
	invited: number;
	not_joined: number;
}

export async function reviveCircle(
	origin: string,
	archive: LoadedArchive,
	blobMap: Record<string, string>,
	sendInvites: boolean
): Promise<ReviveResult> {
	const res = await apiJson<ReviveResult>(origin, '/revive', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			manifest: archive.manifest,
			tables: archive.tables,
			blob_map: blobMap,
			send_invites: sendInvites
		})
	});
	await invalidateSnapshots(origin, { kind: 'circles' });
	return res;
}
