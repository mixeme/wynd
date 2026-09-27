import { apiFetch } from '$lib/api/client';
import {
	getAppSettings,
	getMedia,
	mediaKey,
	putMedia,
	saveAppSettings,
	trimMediaStore
} from '$lib/idb/db';

/**
 * Сколько байт медиа держат живые object URL. Адрес держит свой Blob в памяти
 * вкладки, пока его не отзовут; раньше не отзывался ни один, и за длинную
 * сессию в ленте на тысячи записей копились гигабайты (план 42, MED-1).
 * Сверх бюджета отзывается давно не запрошенный адрес. Уже показанная
 * картинка при этом не пропадает; экран, которому адрес снова нужен, берёт
 * новый через getMediaUrl — из IDB, без сети.
 */
export const URL_CACHE_MAX_BYTES = 256 * 1024 * 1024;

/**
 * Больше этого в IDB не кладём: видео на сотни мегабайт раньше читалось
 * целиком в ArrayBuffer и копировалось в IndexedDB. Такой файл берётся из
 * сети при каждом новом показе.
 */
export const IDB_MEDIA_MAX_BYTES = 20 * 1024 * 1024;

/**
 * Потолок всего кэша медиа в IndexedDB. Раньше общего потолка не было —
 * только на один файл, и кэш рос без конца. Сверх потолка вытесняются давно
 * не открытые файлы; меняется в «Настройки → Приложение».
 */
export const MEDIA_CACHE_DEFAULT_BYTES = 2 * 1024 * 1024 * 1024;

let cacheLimit: number | undefined;
let trimTimer: ReturnType<typeof setTimeout> | undefined;

export async function getMediaCacheLimit(): Promise<number> {
	if (cacheLimit === undefined) {
		const settings = await getAppSettings().catch(() => undefined);
		cacheLimit = settings?.media_cache_bytes ?? MEDIA_CACHE_DEFAULT_BYTES;
	}
	return cacheLimit;
}

/** Новый потолок: сохраняется и сразу подрезает кэш. */
export async function setMediaCacheLimit(bytes: number): Promise<void> {
	cacheLimit = bytes;
	const settings = (await getAppSettings()) ?? { theme: 'system' as const };
	await saveAppSettings({ ...settings, media_cache_bytes: bytes });
	await trimMediaStore(bytes);
}

/** Подрезка после записи — раз в пару секунд, не на каждый файл ленты. */
function scheduleTrim(): void {
	if (trimTimer) return;
	trimTimer = setTimeout(() => {
		trimTimer = undefined;
		void getMediaCacheLimit()
			.then((limit) => trimMediaStore(limit))
			.catch(() => {});
	}, 2000);
}

interface CachedUrl {
	url: string;
	size: number;
}

// Map хранит порядок вставки: первый ключ — давнее всех запрошенный.
const urlCache = new Map<string, CachedUrl>();
let cachedBytes = 0;
let urlBudget = URL_CACHE_MAX_BYTES;
let idbLimit = IDB_MEDIA_MAX_BYTES;

/** Для тестов: бюджеты в байтах вместо сотен мегабайт. */
export function configureMediaCacheForTest(limits: { urlBytes: number; idbBytes: number }): void {
	urlBudget = limits.urlBytes;
	idbLimit = limits.idbBytes;
}

function forget(key: string): void {
	const entry = urlCache.get(key);
	if (!entry) return;
	URL.revokeObjectURL(entry.url);
	urlCache.delete(key);
	cachedBytes -= entry.size;
}

function remember(key: string, blob: Blob): string {
	forget(key);
	const url = URL.createObjectURL(blob);
	urlCache.set(key, { url, size: blob.size });
	cachedBytes += blob.size;
	for (const oldKey of urlCache.keys()) {
		if (cachedBytes <= urlBudget || oldKey === key) break;
		forget(oldKey);
	}
	return url;
}

function touch(key: string): string | undefined {
	const entry = urlCache.get(key);
	if (!entry) return undefined;
	urlCache.delete(key);
	urlCache.set(key, entry);
	return entry.url;
}

export async function getMediaUrl(origin: string, blobId: string): Promise<string> {
	const key = mediaKey(origin, blobId);
	const cached = touch(key);
	if (cached) return cached;

	const stored = await getMedia(key);
	if (stored) {
		return remember(key, new Blob([stored.buffer], { type: stored.mime }));
	}

	const res = await apiFetch(origin, `/blobs/${blobId}`);
	const mime = res.headers.get('Content-Type') || 'application/octet-stream';
	// Blob, а не arrayBuffer(): браузер держит его вне кучи JS.
	const blob = await res.blob();
	const typed = blob.type ? blob : new Blob([blob], { type: mime });
	if (typed.size <= idbLimit) {
		await putMedia(key, { buffer: await typed.arrayBuffer(), mime });
		scheduleTrim();
	}
	return remember(key, typed);
}

export async function downloadBlob(
	origin: string,
	blobId: string,
	filename: string
): Promise<void> {
	const url = await getMediaUrl(origin, blobId);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	a.click();
}

export function revokeMediaUrl(origin: string, blobId: string): void {
	forget(mediaKey(origin, blobId));
}

/** Cache a blob we just uploaded so the UI can show it without a stale entry. */
export function seedMediaUrl(
	origin: string,
	blobId: string,
	data: ArrayBuffer,
	mime: string
): string {
	const key = mediaKey(origin, blobId);
	const url = remember(key, new Blob([data], { type: mime }));
	if (data.byteLength <= idbLimit) void putMedia(key, { buffer: data, mime }).then(scheduleTrim);
	return url;
}

/** Для тестов: сколько байт держат живые адреса. */
export function cachedMediaBytes(): number {
	return cachedBytes;
}
