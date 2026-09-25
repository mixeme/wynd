import { ApiError } from '$lib/api/client';

/** Сбой сети или недоступный сервер — в очередь; ответ API (4xx/5xx) и отмена запроса — нет. */
export function isTransportError(err: unknown): boolean {
	if (err instanceof ApiError) return false;
	if (err instanceof Error && err.name === 'AbortError') return false;
	return true;
}
