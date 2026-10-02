import { openDB, type DBSchema, type IDBPDatabase } from 'idb';
import type { CircleColor } from '$lib/theme/colors';

export type Theme = 'system' | 'light' | 'dark';

export interface SessionRecord {
	origin: string;
	name: string;
	email: string;
	token: string;
	account_id: string;
	/** Когда учётка сохранена на этом origin (логин / код). Старые записи без поля. */
	signed_in_at?: string;
}

export interface AdminSessionRecord {
	origin: string;
	token: string;
}

export interface CursorRecord {
	origin: string;
	seq: number;
}

export interface SnapshotRecord {
	data: unknown;
	fetched_at: number;
}

export interface QueueFile {
	name: string;
	type: string;
	size: number;
	data: ArrayBuffer;
}

export type QueueState = 'pending' | 'uploading' | 'failed';

export type QueueItemType = 'post' | 'comment' | 'reaction' | 'reaction_remove';

export interface QueueMediaMeta {
	kind: string;
	captured_at?: string;
	geo_lat?: number;
	geo_lng?: number;
	is_cover?: boolean;
	audio_artist?: string;
	audio_title?: string;
	audio_cover_blob_id?: string;
	/** JPEG обложки, пока он ещё не загружен на сервер. */
	audio_cover?: { type: string; data: ArrayBuffer };
	/** Голосовое, записанное в приложении (C14): волна 0–100 и длительность. */
	voice?: boolean;
	audio_duration_ms?: number;
	audio_peaks?: number[];
	/** Кадр обложки для ленты (4.16). */
	crop?: { x: number; y: number; w: number; h: number };
}

export interface PostQueuePayload {
	body: string;
	entry_date: string;
	captured_at?: string;
	geo_lat?: number;
	geo_lng?: number;
	media_meta?: QueueMediaMeta[];
}

export interface CommentQueuePayload {
	post_id: string;
	body: string;
}

export interface ReactionQueuePayload {
	post_id: string;
	emoji: string;
}

export interface ReactionRemoveQueuePayload {
	post_id: string;
}

export interface QueueUploadProgress {
	file_index: number;
	session_id: string;
	blob_id?: string;
}

export interface QueueRecord {
	type: QueueItemType;
	origin: string;
	circle_id: string;
	/**
	 * Ключ идемпотентности: сервер по нему узнаёт повтор и возвращает уже
	 * созданную запись вместо дубля (CLI-2). У старых записей очереди поля
	 * нет — они уходят как раньше.
	 */
	client_id?: string;
	/** Сколько раз отправка уже срывалась: задаёт паузу перед следующей (QUE-1). */
	attempts?: number;
	/** Когда запись помечена uploading — чтобы снять зависшую после падения вкладки. */
	uploading_at?: number;
	payload:
		| PostQueuePayload
		| CommentQueuePayload
		| ReactionQueuePayload
		| ReactionRemoveQueuePayload;
	files: QueueFile[];
	state: QueueState;
	uploads?: QueueUploadProgress[];
	error?: string;
	created_at: number;
}

export type QueueRecordWithId = QueueRecord & { id: number };

export interface MediaRecord {
	buffer: ArrayBuffer;
	mime: string;
}

/**
 * Размер и давность файла кэша — отдельно от самих байт: посчитать кэш или
 * выбрать, что вытеснить, раньше значило прочитать все файлы целиком.
 */
export interface MediaMeta {
	size: number;
	/** Когда файл брали в последний раз (мс); 0 — до версии 3, давнее всех. */
	used: number;
}

export interface PinRecord {
	pinned_at: number;
}

export interface GroupRecord {
	id: string;
	name: string;
	circleIds: string[];
	collapsed: boolean;
}

export interface CircleMeta {
	color?: CircleColor;
	identity_name?: string;
}

export interface NotifyDefaults {
	posts?: boolean;
	comments?: boolean;
	comments_mine?: boolean;
	comments_all?: boolean;
	reactions?: boolean;
	events?: boolean;
	mute_until?: string | null;
}

export interface AppSettings {
	theme: Theme;
	notify_defaults?: NotifyDefaults;
	circle_meta?: Record<string, CircleMeta>;
	day_prompt_seen?: Record<string, true>;
	day_prompt_count?: Record<string, number>;
	/** Потолок кэша медиа, байты; нет — умолчание (2 ГБ). */
	media_cache_bytes?: number;
}

interface WyndDB extends DBSchema {
	sessions: {
		key: string;
		value: SessionRecord;
	};
	admin_session: {
		key: 'admin';
		value: AdminSessionRecord;
	};
	cursors: {
		key: string;
		value: CursorRecord;
	};
	snapshots: {
		key: string;
		value: SnapshotRecord;
	};
	queue: {
		key: number;
		value: QueueRecord;
		autoIncrement: true;
	};
	media: {
		key: string;
		value: MediaRecord;
	};
	media_meta: {
		key: string;
		value: MediaMeta;
	};
	pins: {
		key: string;
		value: PinRecord;
	};
	groups: {
		key: string;
		value: GroupRecord;
	};
	settings: {
		key: 'app';
		value: AppSettings;
	};
}

const DB_NAME = 'wynd';
const DB_VERSION = 3;

let dbPromise: Promise<IDBPDatabase<WyndDB>> | undefined;

export async function closeDb(): Promise<void> {
	if (dbPromise) {
		const db = await dbPromise;
		db.close();
		dbPromise = undefined;
	}
}

export function getDb(): Promise<IDBPDatabase<WyndDB>> {
	if (!dbPromise) {
		dbPromise = openDB<WyndDB>(DB_NAME, DB_VERSION, {
			upgrade(db, oldVersion, _newVersion, transaction) {
				if (oldVersion < 1) {
					db.createObjectStore('sessions', { keyPath: 'origin' });
					db.createObjectStore('admin_session');
					db.createObjectStore('cursors', { keyPath: 'origin' });
					db.createObjectStore('snapshots');
					db.createObjectStore('queue', { autoIncrement: true });
					db.createObjectStore('media');
					db.createObjectStore('pins');
					db.createObjectStore('settings');
				}
				if (oldVersion < 2) {
					db.createObjectStore('groups', { keyPath: 'id' });
				}
				if (oldVersion < 3) {
					// Метаданные уже лежащего кэша — один раз при обновлении; used 0:
					// старые файлы вытесняются первыми.
					const meta = db.createObjectStore('media_meta');
					void (async () => {
						let cursor = await transaction.objectStore('media').openCursor();
						while (cursor) {
							await meta.put({ size: cursor.value.buffer.byteLength, used: 0 }, cursor.key);
							cursor = await cursor.continue();
						}
					})();
				}
			},
			// Открытая старая вкладка держит прежнюю версию: без blocked
			// подъём версии вешал новую вкладку молча (LEG-2).
			blocked() {
				console.warn('wynd: обновление хранилища ждёт закрытия других вкладок');
			},
			blocking() {
				void closeDb();
			},
			terminated() {
				dbPromise = undefined;
			}
		}).catch((err) => {
			// Отклонённый промис кэшировать нельзя: в приватном режиме одна
			// неудача запирала ядро до перезагрузки (LEG-2).
			dbPromise = undefined;
			throw err;
		});
	}
	return dbPromise;
}

// Виды снимков: тот же список, что SnapshotKind в $lib/api/snapshots.
// Нужен здесь, чтобы отделить вид от идентификатора в ключе (CLI-1).
const SNAPSHOT_KINDS = new Set([
	'circles',
	'feed',
	'grid',
	'map',
	'days',
	'day',
	'search',
	'instance'
]);

export function snapshotKey(origin: string, kind: string, id: string): string {
	return `${origin}:${kind}:${id}`;
}

export function mediaKey(origin: string, blobId: string): string {
	return `${origin}:${blobId}`;
}

export function pinKey(origin: string, circleId: string): string {
	return `${origin}:${circleId}`;
}

export async function getAppSettings(): Promise<AppSettings | undefined> {
	const db = await getDb();
	return db.get('settings', 'app');
}

export async function saveAppSettings(settings: AppSettings): Promise<void> {
	const db = await getDb();
	await db.put('settings', settings, 'app');
}

function dayPromptKey(origin: string, circleId: string): string {
	return `${origin}:${circleId}`;
}

export async function isDayPromptDismissed(origin: string, circleId: string): Promise<boolean> {
	const settings = await getAppSettings();
	const key = dayPromptKey(origin, circleId);
	return Boolean(settings?.day_prompt_seen?.[key]);
}

export async function getDayPromptShowCount(origin: string, circleId: string): Promise<number> {
	const settings = await getAppSettings();
	const key = dayPromptKey(origin, circleId);
	return settings?.day_prompt_count?.[key] ?? 0;
}

export async function markDayPromptSeen(origin: string, circleId: string): Promise<void> {
	const settings = (await getAppSettings()) ?? { theme: 'system' as Theme };
	const key = dayPromptKey(origin, circleId);
	settings.day_prompt_seen = { ...settings.day_prompt_seen, [key]: true };
	await saveAppSettings(settings);
}

export async function bumpDayPromptCount(origin: string, circleId: string): Promise<void> {
	const settings = (await getAppSettings()) ?? { theme: 'system' as Theme };
	const key = dayPromptKey(origin, circleId);
	const count = (settings.day_prompt_count?.[key] ?? 0) + 1;
	settings.day_prompt_count = { ...settings.day_prompt_count, [key]: count };
	await saveAppSettings(settings);
}

export async function listSessions(): Promise<SessionRecord[]> {
	const db = await getDb();
	return db.getAll('sessions');
}

export async function getSession(origin: string): Promise<SessionRecord | undefined> {
	const db = await getDb();
	return db.get('sessions', origin);
}

export async function putSession(session: SessionRecord): Promise<void> {
	const db = await getDb();
	await db.put('sessions', session);
}

export async function deleteSession(origin: string): Promise<void> {
	const db = await getDb();
	await db.delete('sessions', origin);
}

export async function getAdminSession(): Promise<AdminSessionRecord | undefined> {
	const db = await getDb();
	return db.get('admin_session', 'admin');
}

export async function putAdminSession(session: AdminSessionRecord): Promise<void> {
	const db = await getDb();
	await db.put('admin_session', session, 'admin');
}

export async function clearAdminSession(): Promise<void> {
	const db = await getDb();
	await db.delete('admin_session', 'admin');
}

export async function getCursor(origin: string): Promise<CursorRecord | undefined> {
	const db = await getDb();
	return db.get('cursors', origin);
}

export async function putCursor(origin: string, seq: number): Promise<void> {
	const db = await getDb();
	await db.put('cursors', { origin, seq });
}

export async function deleteCursor(origin: string): Promise<void> {
	const db = await getDb();
	await db.delete('cursors', origin);
}

export async function getSnapshot(key: string): Promise<SnapshotRecord | undefined> {
	const db = await getDb();
	return db.get('snapshots', key);
}

export async function putSnapshot(key: string, record: SnapshotRecord): Promise<void> {
	const db = await getDb();
	await db.put('snapshots', record, key);
}

export async function deleteSnapshot(key: string): Promise<void> {
	const db = await getDb();
	await db.delete('snapshots', key);
}

export interface InvalidateSnapshotsOptions {
	circleId?: string;
	kind?: string;
}

export async function addQueueItem(record: QueueRecord): Promise<number> {
	const db = await getDb();
	return db.add('queue', record);
}

export async function getQueueItem(id: number): Promise<QueueRecord | undefined> {
	const db = await getDb();
	return db.get('queue', id);
}

export async function putQueueItem(id: number, record: QueueRecord): Promise<void> {
	const db = await getDb();
	await db.put('queue', record, id);
}

export async function deleteQueueItem(id: number): Promise<void> {
	const db = await getDb();
	await db.delete('queue', id);
}

export async function listQueueItems(): Promise<QueueRecordWithId[]> {
	const db = await getDb();
	const keys = await db.getAllKeys('queue');
	const items: QueueRecordWithId[] = [];
	for (const key of keys) {
		const record = await db.get('queue', key);
		if (record) items.push({ ...record, id: key as number });
	}
	items.sort((a, b) => b.created_at - a.created_at);
	return items;
}

export async function listQueueForCircle(
	origin: string,
	circleId: string
): Promise<QueueRecordWithId[]> {
	const all = await listQueueItems();
	return all.filter((item) => item.origin === origin && item.circle_id === circleId);
}

export interface PinRow {
	origin: string;
	circleId: string;
	pinned_at: number;
}

export async function getPin(
	origin: string,
	circleId: string
): Promise<PinRecord | undefined> {
	const db = await getDb();
	return db.get('pins', pinKey(origin, circleId));
}

export async function putPin(origin: string, circleId: string): Promise<void> {
	const db = await getDb();
	await db.put('pins', { pinned_at: Date.now() }, pinKey(origin, circleId));
}

export async function deletePin(origin: string, circleId: string): Promise<void> {
	const db = await getDb();
	await db.delete('pins', pinKey(origin, circleId));
}

export async function listGroups(): Promise<GroupRecord[]> {
	const db = await getDb();
	return db.getAll('groups');
}

export async function putGroup(group: GroupRecord): Promise<void> {
	const db = await getDb();
	await db.put('groups', group);
}

export async function deleteGroup(id: string): Promise<void> {
	const db = await getDb();
	await db.delete('groups', id);
}

export async function listPins(): Promise<PinRow[]> {
	const db = await getDb();
	const keys = await db.getAllKeys('pins');
	const rows: PinRow[] = [];
	for (const key of keys) {
		const record = await db.get('pins', key);
		if (!record) continue;
		const raw = key as string;
		const sep = raw.lastIndexOf(':');
		const origin = sep >= 0 ? raw.slice(0, sep) : raw;
		const circleId = sep >= 0 ? raw.slice(sep + 1) : '';
		rows.push({ origin, circleId, pinned_at: record.pinned_at });
	}
	return rows;
}

export async function getMedia(key: string): Promise<MediaRecord | undefined> {
	const db = await getDb();
	const record = await db.get('media', key);
	if (record) {
		// Давность для вытеснения; сбой отметки не мешает показу.
		void db.put('media_meta', { size: record.buffer.byteLength, used: Date.now() }, key).catch(() => {});
	}
	return record;
}

export async function putMedia(key: string, record: MediaRecord): Promise<void> {
	const db = await getDb();
	const tx = db.transaction(['media', 'media_meta'], 'readwrite');
	await tx.objectStore('media').put(record, key);
	await tx.objectStore('media_meta').put({ size: record.buffer.byteLength, used: Date.now() }, key);
	await tx.done;
}

/** Размер кэша медиа — по метаданным, не читая сами файлы. */
export async function mediaStoreBytes(): Promise<number> {
	const db = await getDb();
	let total = 0;
	let cursor = await db.transaction('media_meta').store.openCursor();
	while (cursor) {
		total += cursor.value.size;
		cursor = await cursor.continue();
	}
	return total;
}

/**
 * Держит кэш медиа в потолке: сверх него удаляет давно не открытые файлы,
 * пока не останется 90% потолка (запас, чтобы не чистить на каждой записи).
 * Возвращает, сколько файлов удалено.
 */
export async function trimMediaStore(limitBytes: number): Promise<number> {
	const db = await getDb();
	const tx = db.transaction(['media', 'media_meta'], 'readwrite');
	const meta = tx.objectStore('media_meta');
	const entries: { key: string; size: number; used: number }[] = [];
	let total = 0;
	let cursor = await meta.openCursor();
	while (cursor) {
		entries.push({ key: cursor.key, ...cursor.value });
		total += cursor.value.size;
		cursor = await cursor.continue();
	}
	let removed = 0;
	if (total > limitBytes) {
		entries.sort((a, b) => a.used - b.used);
		for (const entry of entries) {
			if (total <= limitBytes * 0.9) break;
			await tx.objectStore('media').delete(entry.key);
			await meta.delete(entry.key);
			total -= entry.size;
			removed += 1;
		}
	}
	await tx.done;
	return removed;
}

export async function clearMediaStore(): Promise<void> {
	const db = await getDb();
	await db.clear('media');
	await db.clear('media_meta');
}

/**
 * Стирает всё локальное состояние одного сервера, кроме сессии, курсора и
 * снимков (их снимает dropParticipantSession отдельно): очередь, кэш медиа,
 * закреплённые круги. Без этого после выхода на общем устройстве очередь
 * уходила под следующей учёткой, а фото оставались в IDB (аудит 2026-09-22).
 */
export async function clearOriginState(origin: string): Promise<void> {
	const db = await getDb();
	const prefix = `${origin}:`;
	const queue = db.transaction('queue', 'readwrite');
	let q = await queue.store.openCursor();
	while (q) {
		if (q.value.origin === origin) await q.delete();
		q = await q.continue();
	}
	await queue.done;
	for (const store of ['media', 'media_meta', 'pins'] as const) {
		const tx = db.transaction(store, 'readwrite');
		let cursor = await tx.store.openCursor();
		while (cursor) {
			if ((cursor.key as string).startsWith(prefix)) await cursor.delete();
			cursor = await cursor.continue();
		}
		await tx.done;
	}
}

/**
 * Ключ снимка — `${origin}:${kind}:${id}`. Без `kind` остаток после
 * `${origin}:` начинается с вида снимка (`feed:<uuid>`), поэтому прежняя
 * сверка «остаток === circleId» не совпадала никогда: после исключения из
 * круга, правки и удаления записи кэш круга оставался и читался офлайн
 * (CLI-1). Теперь вид отрезается и обязан быть известным — это же отделяет
 * два сервера на одном хосте с разными портами.
 */
export async function invalidateSnapshots(
	origin: string,
	opts: InvalidateSnapshotsOptions = {}
): Promise<void> {
	const db = await getDb();
	const tx = db.transaction('snapshots', 'readwrite');
	const prefix = opts.kind ? `${origin}:${opts.kind}:` : `${origin}:`;

	let cursor = await tx.store.openCursor();
	while (cursor) {
		const key = cursor.key as string;
		let remove = key.startsWith(prefix);
		if (remove && opts.circleId) {
			let idPart = key.slice(prefix.length);
			if (!opts.kind) {
				const sep = idPart.indexOf(':');
				const kind = sep === -1 ? '' : idPart.slice(0, sep);
				if (!SNAPSHOT_KINDS.has(kind)) {
					remove = false;
				} else {
					idPart = idPart.slice(sep + 1);
				}
			}
			if (remove) {
				remove = idPart === opts.circleId || idPart.startsWith(`${opts.circleId}:`);
			}
		}
		if (remove) {
			await cursor.delete();
		}
		cursor = await cursor.continue();
	}
	await tx.done;
}
