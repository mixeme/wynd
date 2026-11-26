/**
 * Последний опубликованный выпуск приложения Wynd для Android (C139).
 * Файл лежит в выпусках GitHub — оттуда же приложение обновляется само
 * (план 50). Страница установки спрашивает адрес файла в момент нажатия:
 * кнопка всегда одна и та же, а выпуск берётся самый новый.
 *
 * GitHub недоступен или ответ не разобрался — остаётся страница выпусков:
 * человек дойдёт до APK сам.
 */
export const ANDROID_RELEASES_PAGE = 'https://github.com/mixeme/wynd/releases?q=android-v';

const RELEASES_API = 'https://api.github.com/repos/mixeme/wynd/releases?per_page=100';
const TAG_PREFIX = 'android-v';
const APK_ASSET = 'wynd-android.apk';

export interface AndroidAppRelease {
	/** Версия без метки: «0.1.5». */
	version: string;
	/** Прямая ссылка на подписанный APK. */
	apkUrl: string;
	/** Страница выпуска — что нового и суммы файлов. */
	pageUrl: string;
}

interface GitHubRelease {
	tag_name?: string;
	draft?: boolean;
	prerelease?: boolean;
	html_url?: string;
	assets?: { name?: string; browser_download_url?: string }[];
}

function versionOf(tag: string): string | undefined {
	if (!tag.startsWith(TAG_PREFIX)) return undefined;
	const version = tag.slice(TAG_PREFIX.length);
	const parts = version.split('.');
	if (parts.length !== 3) return undefined;
	for (const part of parts) {
		if (!/^\d{1,6}$/.test(part)) return undefined;
	}
	return version;
}

function older(a: string, b: string): boolean {
	const pa = a.split('.').map(Number);
	const pb = b.split('.').map(Number);
	for (let i = 0; i < 3; i++) {
		if (pa[i] !== pb[i]) return pa[i] < pb[i];
	}
	return false;
}

/** Самый новый опубликованный выпуск Android с файлом APK. */
export function pickAndroidAppRelease(releases: GitHubRelease[]): AndroidAppRelease | undefined {
	let best: AndroidAppRelease | undefined;
	for (const release of releases) {
		if (release.draft || release.prerelease || !release.tag_name) continue;
		const version = versionOf(release.tag_name);
		if (!version) continue;
		const apkUrl = release.assets?.find((asset) => asset.name === APK_ASSET)?.browser_download_url;
		if (!apkUrl) continue;
		if (best && !older(best.version, version)) continue;
		best = { version, apkUrl, pageUrl: release.html_url ?? ANDROID_RELEASES_PAGE };
	}
	return best;
}

export async function fetchLatestAndroidAppRelease(
	fetchImpl: typeof fetch = fetch,
	timeoutMs = 6000
): Promise<AndroidAppRelease | undefined> {
	// Ответа может не быть вовсе: тогда человек идёт на страницу выпусков.
	const controller = typeof AbortController === 'undefined' ? undefined : new AbortController();
	const timer = controller ? setTimeout(() => controller.abort(), timeoutMs) : undefined;
	try {
		const res = await fetchImpl(RELEASES_API, {
			headers: { Accept: 'application/vnd.github+json' },
			signal: controller?.signal
		});
		if (!res.ok) return undefined;
		const releases = (await res.json()) as GitHubRelease[];
		if (!Array.isArray(releases)) return undefined;
		return pickAndroidAppRelease(releases);
	} catch {
		return undefined;
	} finally {
		if (timer) clearTimeout(timer);
	}
}
