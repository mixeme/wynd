import { goto } from '$app/navigation';
import { ApiError, apiJson, apiPath, normalizeOrigin } from '$lib/api/client';
import { openDownloadAfter } from '$lib/navigation/openDownload';

interface DownloadState {
	pending?: boolean;
	retryAt?: number;
	error?: string;
}

const states = $state<Record<string, DownloadState>>({});
// Repeated taps on the same archive share a single link request.
const requests = new Map<string, Promise<void>>();

function key(origin: string, circleId: string): string {
	return JSON.stringify([normalizeOrigin(origin), circleId]);
}

export function archiveDownloadState(origin: string, circleId: string): DownloadState | undefined {
	return states[key(origin, circleId)];
}

export function archiveRetrySeconds(value: unknown): number {
	const seconds = Number(value);
	return Number.isFinite(seconds) && seconds >= 1 && seconds <= 3600 ? Math.ceil(seconds) : 30;
}

function secondsText(seconds: number): string {
	const n = seconds % 100;
	const last = seconds % 10;
	return `${seconds} ${n >= 11 && n <= 14 ? 'секунд' : last === 1 ? 'секунду' : last >= 2 && last <= 4 ? 'секунды' : 'секунд'}`;
}

/** Text of the approved 6.23a/b; zero means the retry interval has elapsed. */
export function archiveBusyMessage(seconds: number, standalone = false): string {
	const wait = seconds > 0 ? ` через ${secondsText(seconds)}` : '';
	return standalone
		? `Дождитесь окончания скачивания. Если оно прервалось, попробуйте снова в Wynd${wait}.`
		: `Скачивание уже началось. Дождитесь его окончания. Если оно прервалось, попробуйте снова${wait}.`;
}

/** Ask with the session; let the browser download by its one-time token. */
export function startArchiveDownload(origin: string, downloadUrl: string): Promise<void> {
	const server = normalizeOrigin(origin);
	const path = downloadUrl.replace(/^\/api\/v1/, '');
	const match = /^\/circles\/([^/]+)\/archive\/download$/.exec(path);
	if (!match) return Promise.reject(new Error('invalid_archive_url'));
	const circleId = decodeURIComponent(match[1]);
	const stateKey = key(server, circleId);
	const pending = requests.get(stateKey);
	if (pending) return pending;
	const current = states[stateKey];
	if (current?.retryAt && current.retryAt > Date.now()) return showArchive(circleId);
	states[stateKey] = { pending: true };
	const request = (async () => {
		try {
			await openDownloadAfter(async () => {
				const { url } = await apiJson<{ url: string }>(
					origin,
					path.replace(/\/download$/, '/link'),
					{
						method: 'POST'
					}
				);
				return apiPath(origin, url.replace(/^\/api\/v1/, ''));
			});
			delete states[stateKey];
		} catch (err) {
			if (err instanceof ApiError && err.status === 429 && err.code === 'rate_limited') {
				states[stateKey] = { retryAt: Date.now() + archiveRetrySeconds(err.retryAfterSec) * 1000 };
			} else {
				// Loaded only after failure: auth also imports sync/posts.
				const { authErrorHint } = await import('$lib/auth/auth');
				states[stateKey] = { error: authErrorHint(err) };
			}
			await showArchive(circleId);
		} finally {
			requests.delete(stateKey);
		}
	})();
	requests.set(stateKey, request);
	return request;
}

async function showArchive(circleId: string): Promise<void> {
	const path = `/circles/${encodeURIComponent(circleId)}/archive`;
	if (window.location.pathname !== path) await goto(path);
}
