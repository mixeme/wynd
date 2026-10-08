import { apiJson } from '$lib/api/client';
import type { CircleSearchHit, CircleSearchResponse } from './types';

export interface SearchFilters {
	from?: string;
	to?: string;
	hasPhoto?: boolean;
	hasLocation?: boolean;
	author?: string;
}

function appendFilters(params: URLSearchParams, filters?: SearchFilters) {
	if (!filters) return;
	if (filters.from) params.set('from', filters.from);
	if (filters.to) params.set('to', filters.to);
	if (filters.hasPhoto) params.set('has_photo', '1');
	if (filters.hasLocation) params.set('has_location', '1');
	if (filters.author) params.set('author', filters.author);
}

export async function searchCircle(
	origin: string,
	circleId: string,
	query: string,
	limit = 50,
	filters?: SearchFilters
): Promise<CircleSearchHit[]> {
	const params = new URLSearchParams({ q: query, limit: String(limit) });
	appendFilters(params, filters);
	const data = await apiJson<CircleSearchResponse>(
		origin,
		`/circles/${circleId}/search?${params}`
	);
	return data.hits ?? [];
}

export async function searchCircleAuthors(
	origin: string,
	circleId: string,
	query: string,
	filters?: Omit<SearchFilters, 'author'>
): Promise<string[]> {
	const params = new URLSearchParams({ q: query });
	appendFilters(params, filters);
	const data = await apiJson<{ authors: string[] }>(
		origin,
		`/circles/${circleId}/search/authors?${params}`
	);
	return data.authors ?? [];
}

/** Chip state on `/search` and `/circles/[id]/search` (UI query, not API `has_photo`). */
export type SearchChipState = {
	q?: string;
	periodFrom?: string;
	periodTo?: string;
	periodActive?: boolean;
	hasPhoto?: boolean;
	hasLocation?: boolean;
	author?: string;
};

export function searchHref(path: string, state: SearchChipState): string {
	const params = new URLSearchParams();
	const q = state.q?.trim();
	if (q) params.set('q', q);
	if (state.periodActive && state.periodFrom) params.set('from', state.periodFrom);
	if (state.periodActive && state.periodTo) params.set('to', state.periodTo);
	if (state.hasPhoto) params.set('photo', '1');
	if (state.hasLocation) params.set('location', '1');
	const author = state.author?.trim();
	if (author) params.set('author', author);
	const qs = params.toString();
	return qs ? `${path}?${qs}` : path;
}

/** Найденное слово в кавычках-ёлочках: «как найдено» — без разметки, текстом. */
/**
 * Куда ведёт находка: день — в день, комментарий и файл из комментария — к
 * реплике в обсуждение, файл или звук записи — к его строке, запись — в запись. Экран записи встаёт на
 * найденное и на миг подсвечивает, как отклик из «Откликов».
 */
export function searchHitHref(
	circleId: string,
	hit: { kind: string; post_id: string; entry_date: string; comment_id?: string; media_blob_id?: string }
): string {
	const base = `/circles/${circleId}`;
	if (hit.kind === 'day') return `${base}/days/${hit.entry_date}`;
	const post = `${base}/posts/${hit.post_id}`;
	// Файл из комментария приходит с его id — ведём к реплике.
	if (hit.comment_id && (hit.kind === 'comment' || hit.kind === 'file' || hit.kind === 'audio')) {
		return `${post}?comment=${hit.comment_id}`;
	}
	if ((hit.kind === 'file' || hit.kind === 'audio') && hit.media_blob_id) {
		return `${post}?media=${hit.media_blob_id}`;
	}
	return `${post}?found=1`;
}

export function quoteMatch(text: string, term: string): string {
	if (!term) return text;
	const idx = text.toLowerCase().indexOf(term.toLowerCase());
	if (idx < 0) return text;
	return `${text.slice(0, idx)}«${text.slice(idx, idx + term.length)}»${text.slice(idx + term.length)}`;
}

/** Stable placeholder tint when a hit has media but no cached blob URL yet. */
export function searchThumbVariant(seed: string): string {
	let hash = 0;
	for (let i = 0; i < seed.length; i++) {
		hash = (hash * 31 + seed.charCodeAt(i)) | 0;
	}
	return `p${(Math.abs(hash) % 8) + 1}`;
}

export function searchLocation(pathname: string, state: SearchChipState): string {
	return searchHref(pathname, state);
}

/** Sync chip state into the address bar without SvelteKit navigation (avoids query reset loops). */
export function replaceSearchUrl(pathname: string, state: SearchChipState): void {
	if (typeof window === 'undefined') return;
	const href = searchLocation(pathname, state);
	const current = `${pathname}${window.location.search}`;
	if (href !== current) {
		history.replaceState(history.state, '', href);
	}
}

export function searchChipsFromWindow(): ReturnType<typeof searchChipsFromParams> {
	return searchChipsFromParams(new URL(window.location.href).searchParams);
}

export function searchChipsFromParams(params: URLSearchParams): {
	q: string;
	from: string;
	to: string;
	hasPhoto: boolean;
	hasLocation: boolean;
	author: string;
} {
	const from = params.get('from') ?? '';
	const to = params.get('to') ?? '';
	return {
		q: params.get('q') ?? '',
		from,
		to,
		hasPhoto: params.get('photo') === '1' || params.get('has_photo') === '1',
		hasLocation: params.get('location') === '1' || params.get('has_location') === '1',
		author: params.get('author') ?? ''
	};
}
