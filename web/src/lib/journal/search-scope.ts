import type { CircleSeed } from '$lib/circles/seed';

type SearchCircle = Omit<CircleSeed, 'at'>;
let circle: SearchCircle | undefined;
let destination: string | undefined;

function isCircleSearch(url: URL, id: string | null): boolean {
	return Boolean(id && url.pathname === '/circles/' + encodeURIComponent(id) + '/search');
}

export function searchScopeChanges(from: URL | undefined, to: URL | undefined): boolean {
	if (!from || !to) return false;
	if (from.pathname === '/search') return isCircleSearch(to, from.searchParams.get('circle'));
	return to.pathname === '/search' && isCircleSearch(from, to.searchParams.get('circle'));
}

/** Keep the circle shell only while moving between the two scopes of this search. */
export function trackSearchScope(from: URL | undefined, to: URL | undefined): void {
	destination = searchScopeChanges(from, to) ? to?.pathname : undefined;
	if (!destination) circle = undefined;
}

export function rememberSearchCircle(seed: SearchCircle): void {
	circle = seed;
}

export function searchCircleSeed(path: string, id: string): SearchCircle | undefined {
	if (path === destination && circle?.id === id) return circle;
}

export function searchFocusAfterNavigation(path: string): boolean {
	return path !== destination;
}
