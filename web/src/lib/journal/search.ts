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
	return data.hits;
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
};

export function searchHref(path: string, state: SearchChipState): string {
	const params = new URLSearchParams();
	const q = state.q?.trim();
	if (q) params.set('q', q);
	if (state.periodActive && state.periodFrom) params.set('from', state.periodFrom);
	if (state.periodActive && state.periodTo) params.set('to', state.periodTo);
	if (state.hasPhoto) params.set('photo', '1');
	if (state.hasLocation) params.set('location', '1');
	const qs = params.toString();
	return qs ? `${path}?${qs}` : path;
}

export function searchChipsFromParams(params: URLSearchParams): {
	q: string;
	from: string;
	to: string;
	hasPhoto: boolean;
	hasLocation: boolean;
} {
	const from = params.get('from') ?? '';
	const to = params.get('to') ?? '';
	return {
		q: params.get('q') ?? '',
		from,
		to,
		hasPhoto: params.get('photo') === '1' || params.get('has_photo') === '1',
		hasLocation: params.get('location') === '1' || params.get('has_location') === '1'
	};
}
