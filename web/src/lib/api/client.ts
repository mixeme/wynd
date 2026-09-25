import { getAdminSession, getSession, invalidateSnapshots } from '$lib/idb/db';

export class ApiError extends Error {
	readonly status: number;
	readonly code: string;
	readonly detail?: string;
	readonly retryAfterSec?: number;

	constructor(status: number, code: string, detail?: string, retryAfterSec?: number) {
		super(code);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
		this.detail = detail;
		this.retryAfterSec = retryAfterSec;
	}
}

const circlePathRe = /^\/circles\/([^/]+)/;

export function normalizeOrigin(origin: string): string {
	if (!origin) return '';
	if (typeof window !== 'undefined' && origin === window.location.origin) return '';
	return origin.replace(/\/$/, '');
}

export function apiPath(origin: string, path: string): string {
	const base = normalizeOrigin(origin);
	const suffix = path.startsWith('/') ? path : `/${path}`;
	const api = `/api/v1${suffix}`;
	return base ? `${base}${api}` : api;
}

async function resolveToken(origin: string, path: string): Promise<string | undefined> {
	if (path.startsWith('/admin')) {
		return (await getAdminSession())?.token;
	}
	return (await getSession(origin))?.token;
}

async function parseApiError(res: Response): Promise<ApiError> {
	let retryAfterSec: number | undefined;
	const retryHeader = res.headers.get('Retry-After');
	if (retryHeader) {
		const parsed = Number.parseInt(retryHeader, 10);
		if (!Number.isNaN(parsed)) retryAfterSec = parsed;
	}
	try {
		const body: unknown = await res.json();
		if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
			const detail =
				'detail' in body && typeof body.detail === 'string' ? body.detail : undefined;
			if (
				'retry_after_sec' in body &&
				typeof (body as { retry_after_sec?: number }).retry_after_sec === 'number'
			) {
				retryAfterSec = (body as { retry_after_sec: number }).retry_after_sec;
			}
			return new ApiError(res.status, body.error, detail, retryAfterSec);
		}
	} catch {
		/* not JSON */
	}
	return new ApiError(res.status, 'unknown', undefined, retryAfterSec);
}

export async function apiFetch(
	origin: string,
	path: string,
	init: RequestInit = {}
): Promise<Response> {
	const headers = new Headers(init.headers);
	const token = await resolveToken(origin, path);
	if (token) headers.set('Authorization', `Bearer ${token}`);
	if (!headers.has('Accept') && !path.startsWith('/blobs/')) {
		headers.set('Accept', 'application/json');
	}

	const res = await fetch(apiPath(origin, path), { ...init, headers });
	if (!res.ok) {
		const err = await parseApiError(res);
		if (isPaymentRequired(err)) {
			if (typeof window !== 'undefined' && !path.startsWith('/admin')) {
				window.location.assign('/circles');
			}
		} else if (err.status === 403) {
			const match = path.match(circlePathRe);
			if (match) {
				await invalidateSnapshots(origin, { circleId: match[1] });
			}
		}
		throw err;
	}
	return res;
}

export function isPaymentRequired(err: unknown): err is ApiError & { code: 'payment_required' } {
	return err instanceof ApiError && err.status === 403 && err.code === 'payment_required';
}

export function isAccessError(err: unknown): err is ApiError {
	return (
		err instanceof ApiError &&
		(err.status === 403 || err.status === 404) &&
		!isPaymentRequired(err)
	);
}

export async function apiJson<T>(
	origin: string,
	path: string,
	init: RequestInit = {}
): Promise<T> {
	const res = await apiFetch(origin, path, init);
	return res.json() as Promise<T>;
}
