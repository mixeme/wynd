import { apiJson } from '$lib/api/client';
import { dropParticipantSession, isSessionRejected } from '$lib/session/session.svelte';
import { listSessions, listPins, type SessionRecord } from '$lib/idb/db';
import { invalidateSnapshots } from '$lib/idb/db';
import { readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import { getCircleColor, setCircleColor, circleInitial } from './meta';
import { CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
import { formatPostTime } from '$lib/format/time';

export interface ArchiveCycleBanner {
	active: true;
	cutoff_date: string;
	deadline: string;
	reminder_before_sec: number;
	cutoff_locked: boolean;
	personal_archive_bytes: number;
	personal_archive_media_count: number;
	personal_archive_post_count: number;
	download_url: string;
}

export interface CircleListItem {
	id: string;
	name: string;
	color?: string;
	status: string;
	unread: number;
	last_read_seq: number;
	last_summary?: string;
	last_at?: string;
	archive_cycle?: ArchiveCycleBanner;
}

export interface CirclesResponse {
	circles: CircleListItem[];
}

export interface StreetCircle {
	origin: string;
	instanceName: string;
	id: string;
	name: string;
	unread: number;
	pinned: boolean;
	pinnedAt: number;
	color: string;
	initial: string;
	preview: string;
	time: string;
	pendingJoin?: boolean;
}

export interface SearchHit {
	post_id: string;
	comment_id?: string;
	circle_id: string;
	author_name?: string;
	kind: string;
	title?: string;
	snippet: string;
	thumb_blob_id?: string;
	entry_date: string;
	created_at: string;
}

export interface SearchResponse {
	hits: SearchHit[];
}

export interface CreateCircleInput {
	name: string;
	owner_name: string;
	edit_window_sec: number | null;
	color?: CircleColor;
}

export async function fetchCircles(origin: string): Promise<CircleListItem[]> {
	const data = await apiJson<CirclesResponse>(origin, '/circles');
	await writeCachedSnapshot(origin, 'circles', '_list', data);
	return data.circles;
}

export async function fetchPendingCircleJoins(origin: string): Promise<string[]> {
	const data = await apiJson<{ circle_ids?: string[] | null }>(origin, '/pending-circle-joins');
	return data.circle_ids ?? [];
}

export async function loadCirclesCached(origin: string): Promise<CircleListItem[]> {
	const cached = await readCachedSnapshot<CirclesResponse>(origin, 'circles', '_list');
	return cached?.circles ?? [];
}

export async function createCircle(
	origin: string,
	input: CreateCircleInput
): Promise<{ id: string; name: string }> {
	const result = await apiJson<{ id: string; name: string }>(origin, '/circles', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	await invalidateSnapshots(origin, { kind: 'circles' });
	return result;
}

export async function searchOrigin(
	origin: string,
	query: string,
	limit = 50,
	filters?: import('$lib/journal/search').SearchFilters
): Promise<SearchHit[]> {
	const params = new URLSearchParams({ q: query, limit: String(limit) });
	if (filters?.from) params.set('from', filters.from);
	if (filters?.to) params.set('to', filters.to);
	if (filters?.hasPhoto) params.set('has_photo', '1');
	if (filters?.hasLocation) params.set('has_location', '1');
	if (filters?.author) params.set('author', filters.author);
	const data = await apiJson<SearchResponse>(origin, `/search?${params}`);
	return data.hits ?? [];
}

async function colorForCircle(
	origin: string,
	circleId: string,
	serverColor?: string
): Promise<string> {
	if (serverColor && serverColor in CIRCLE_COLORS) {
		await setCircleColor(origin, circleId, serverColor as CircleColor);
		return CIRCLE_COLORS[serverColor as CircleColor].cssVar;
	}
	const stored = await getCircleColor(origin, circleId);
	if (stored) return CIRCLE_COLORS[stored].cssVar;
	const palette = Object.values(CIRCLE_COLORS);
	const hash = circleId.split('').reduce((a, c) => a + c.charCodeAt(0), 0);
	return palette[hash % palette.length].cssVar;
}

export async function loadStreetCircles(): Promise<StreetCircle[]> {
	const sessions = await listSessions();
	const pins = await listPins();
	const pinMap = new Map(pins.map((p) => [`${p.origin}:${p.circleId}`, p.pinned_at]));

	const rows: StreetCircle[] = [];

	for (const session of sessions) {
		let circles: CircleListItem[];
		try {
			circles = await fetchCircles(session.origin);
		} catch (err) {
			if (isSessionRejected(err)) {
				await dropParticipantSession(session.origin);
				continue;
			}
			circles = await loadCirclesCached(session.origin);
		}
		for (const circle of circles) {
			if (circle.status !== 'active' && circle.status !== 'left_with_access') continue;
			const pinKey = `${session.origin}:${circle.id}`;
			const pinnedAt = pinMap.get(pinKey) ?? 0;
			const preview =
				circle.status === 'left_with_access'
					? 'читает, не пишет'
					: (circle.last_summary ??
						(sessions.length > 1 ? session.name : 'Откройте, чтобы посмотреть'));
			const time = circle.last_at ? formatPostTime(circle.last_at, '') : '';
			rows.push({
				origin: session.origin,
				instanceName: session.name,
				id: circle.id,
				name: circle.name,
				unread: circle.status === 'left_with_access' ? 0 : circle.unread,
				pinned: pinnedAt > 0,
				pinnedAt,
				color: await colorForCircle(session.origin, circle.id, circle.color),
				initial: circleInitial(circle.name),
				preview,
				time
			});
		}
		let pendingIds: string[] = [];
		try {
			pendingIds = await fetchPendingCircleJoins(session.origin);
		} catch {
			pendingIds = [];
		}
		for (const id of pendingIds) {
			const existing = rows.find((row) => row.origin === session.origin && row.id === id);
			// Вышедший с доступом уже на полке: без этой отметки повторное
			// «позвать» не открывает вступление, круг просто читается дальше.
			if (existing) {
				if (existing.preview === 'читает, не пишет') {
					existing.pendingJoin = true;
					existing.preview = 'Вас снова позвали — выберите имя';
					existing.unread = 1;
				}
				continue;
			}
			try {
				const preview = await apiJson<{ circle_name: string; color?: string }>(
					session.origin,
					`/circles/${id}/join-preview`
				);
				rows.push({
					origin: session.origin,
					instanceName: session.name,
					id,
					name: preview.circle_name,
					unread: 1,
					pinned: false,
					pinnedAt: 0,
					color: await colorForCircle(session.origin, id, preview.color),
					initial: circleInitial(preview.circle_name),
					preview: 'Вас позвали — выберите имя',
					time: '',
					pendingJoin: true
				});
			} catch {
				/* preview gone */
			}
		}
	}

	rows.sort((a, b) => {
		if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
		if (a.pinned && b.pinned) return b.pinnedAt - a.pinnedAt;
		return a.name.localeCompare(b.name, 'ru');
	});

	return rows;
}

export function ownerNameFromSession(session: SessionRecord): string {
	const local = session.email.split('@')[0]?.trim();
	return local || session.email;
}

export async function circleNameMap(): Promise<Map<string, { name: string; color: string }>> {
	const map = new Map<string, { name: string; color: string }>();
	const rows = await loadStreetCircles();
	for (const row of rows) {
		map.set(`${row.origin}:${row.id}`, { name: row.name, color: row.color });
	}
	return map;
}

export async function searchAllOrigins(
	query: string,
	filters?: import('$lib/journal/search').SearchFilters
): Promise<{
	results: Array<{ origin: string; hits: SearchHit[] }>;
	failedInstanceNames: string[];
}> {
	const sessions = await listSessions();
	const results: Array<{ origin: string; hits: SearchHit[] }> = [];
	const failedInstanceNames: string[] = [];
	await Promise.all(
		sessions.map(async (session) => {
			try {
				const hits = await searchOrigin(session.origin, query, 50, filters);
				if (hits.length) results.push({ origin: session.origin, hits });
			} catch {
				failedInstanceNames.push(session.name);
			}
		})
	);
	return { results, failedInstanceNames };
}

export type { CircleColor };
