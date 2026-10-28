import { apiJson } from '$lib/api/client';
import { dropParticipantSession, isSessionRejected } from '$lib/session/session.svelte';
import { listGroups, listSessions, type SessionRecord } from '$lib/idb/db';
import { invalidateSnapshots } from '$lib/idb/db';
import { readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import { getCircleColor, setCircleColor, circleInitial } from './meta';
import { CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
import { formatPostTime } from '$lib/format/time';
import { serverHost } from './move';
import {
	applyStreetOps,
	flushStreetOps,
	mergeStreetGroups,
	moveDeviceStreetToServers,
	pendingStreetOps,
	settleLocalGroups,
	type ServerGroup,
	type StreetGroupView
} from './street';

export interface ArchiveCycleBanner {
	active: true;
	cutoff_date: string;
	deadline: string;
	reminder_before_sec: number;
	cutoff_locked: boolean;
	personal_archive_bytes: number;
	personal_archive_media_count: number;
	personal_archive_post_count: number;
	/** Нет — срок прошёл, архив уже не отдаётся (C62). */
	download_url?: string;
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
	/** Новые отклики к записям (3.17): на улочке — точка, если записей новых нет. */
	responses_unread?: number;
	/** Места нет (C42): «circle» — упёрлась квота круга (3.10), «server» — потолок сервера (3.11). */
	space_full?: 'circle' | 'server';
	/** Владелец круга — при «circle»: его называет полоса 3.10. */
	owner_name?: string;
	you_own?: boolean;
	/** Круг замер под полный архив с удалением (3.16, 2.5) или под перенос (3.14). */
	frozen_at?: string;
	/** Круг переезжает на другой сервер (план 53): 3.14, после «принято» — 3.15, 2.4. */
	move?: CircleMoveBanner;
	/** Закреплён на улочке (A2), RFC 3339. */
	pinned_at?: string;
}

/** Перенос круга глазами участника: куда и какой почтой входить там. */
export interface CircleMoveBanner {
	status: 'waiting' | 'pulling' | 'accepted';
	dest_url?: string;
	dest_name?: string;
	email?: string;
}

// Что с замершим кругом — вместо последней записи на улочке: переехал (2.4),
// переезжает, уходит с сервера (2.5).
function leavingPreview(circle: CircleListItem): string {
	if (circle.move?.status === 'accepted') {
		return `Круг переехал — войдите на ${serverHost(circle.move.dest_url)}`;
	}
	if (circle.move) return 'Круг замер на время переезда';
	return circle.you_own ? 'Круг уходит с сервера' : 'Круг уходит с сервера — скачайте своё';
}

export interface CirclesResponse {
	circles: CircleListItem[];
	/** Группы учётки на этом сервере (A2); у снимка до A2 — нет. */
	groups?: ServerGroup[];
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
	/** Время последнего события — порядок на улице (RFC 3339, пусто — событий нет). */
	lastAt?: string;
	/** Есть новые отклики — точка у круга (2.1). */
	responses?: boolean;
	pendingJoin?: boolean;
}

export interface SearchHit {
	post_id: string;
	comment_id?: string;
	/** Блоб найденного файла или звука — экран записи подсвечивает его строку. */
	media_blob_id?: string;
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
	/** Пол создателя для строк журнала (A5). */
	owner_gender?: '' | 'm' | 'f';
	edit_window_sec: number | null;
	color?: CircleColor;
}

export async function fetchCircles(origin: string): Promise<CircleListItem[]> {
	return (await fetchCirclesResponse(origin)).circles;
}

async function fetchCirclesResponse(origin: string): Promise<CirclesResponse> {
	const data = await apiJson<CirclesResponse>(origin, '/circles');
	await writeCachedSnapshot(origin, 'circles', '_list', data);
	return data;
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

/**
 * Свежесть (2.1): приглашение, где ещё не выбрано имя, — самое свежее. Круги
 * без событий в конце, по имени. Так стоят и круги внутри группы.
 */
export function byFreshness(a: StreetCircle, b: StreetCircle): number {
	const fresh = (row: StreetCircle) =>
		row.pendingJoin ? Infinity : Date.parse(row.lastAt ?? '') || 0;
	const fa = fresh(a);
	const fb = fresh(b);
	if (fa !== fb) return fb - fa;
	return a.name.localeCompare(b.name, 'ru');
}

/**
 * Порядок улицы (2.1): закреплённые сверху — по времени закрепления, дальше
 * по свежести.
 */
export function sortStreetCircles(rows: StreetCircle[]): StreetCircle[] {
	return rows.sort((a, b) => {
		if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
		if (a.pinned && b.pinned) return b.pinnedAt - a.pinnedAt;
		return byFreshness(a, b);
	});
}

/** Улочка: круги всех серверов и группы, сложенные по названию (A2). */
export interface Street {
	circles: StreetCircle[];
	groups: StreetGroupView[];
}

export async function loadStreetCircles(): Promise<StreetCircle[]> {
	return (await loadStreet()).circles;
}

export async function loadStreet(): Promise<Street> {
	const sessions = await listSessions();
	await moveDeviceStreetToServers(sessions.map((s) => s.origin));

	const rows: StreetCircle[] = [];
	const layouts: Array<{ origin: string; groups: ServerGroup[] }> = [];
	// Ряды «читает, не пишет» — по статусу, а не по тексту превью.
	const readOnly = new Set<StreetCircle>();

	for (const session of sessions) {
		// Изменения раскладки, которых сервер ещё не принял, — сначала: ответ
		// придёт уже с ними. Молчит — останутся в очереди и лягут поверх снимка.
		await flushStreetOps(session.origin).catch(() => {});
		let data: CirclesResponse;
		try {
			data = await fetchCirclesResponse(session.origin);
		} catch (err) {
			if (isSessionRejected(err)) {
				await dropParticipantSession(session.origin);
				continue;
			}
			data = (await readCachedSnapshot<CirclesResponse>(session.origin, 'circles', '_list')) ?? {
				circles: []
			};
		}
		const circles = data.circles ?? [];
		const layout = applyStreetOps(
			{
				pins: new Map(
					circles
						.filter((c) => c.pinned_at)
						.map((c) => [c.id, Date.parse(c.pinned_at ?? '') || 1] as [string, number])
				),
				groups: data.groups ?? []
			},
			await pendingStreetOps(session.origin)
		);
		layouts.push({ origin: session.origin, groups: layout.groups });
		for (const circle of circles) {
			if (circle.status !== 'active' && circle.status !== 'left_with_access') continue;
			const pinnedAt = layout.pins.get(circle.id) ?? 0;
			// Круг уходит с сервера (2.5): что с ним и что можно успеть — вместо
			// последней записи, без времени.
			const leaving = circle.status === 'active' && Boolean(circle.frozen_at);
			const preview = leaving
				? leavingPreview(circle)
				: circle.status === 'left_with_access'
					? 'читает, не пишет'
					: (circle.last_summary ??
						(sessions.length > 1 ? session.name : 'Откройте, чтобы посмотреть'));
			const time = circle.last_at && !leaving ? formatPostTime(circle.last_at) : '';
			const row: StreetCircle = {
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
				time,
				lastAt: circle.last_at,
				responses: circle.status === 'active' && (circle.responses_unread ?? 0) > 0
			};
			rows.push(row);
			if (circle.status === 'left_with_access') readOnly.add(row);
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
				if (readOnly.has(existing)) {
					existing.pendingJoin = true;
					existing.preview = 'Вас снова позвали — выберите имя';
					existing.unread = 1;
				}
				continue;
			}
			try {
				const preview = await apiJson<{
					circle_name: string;
					color?: string;
					split?: { invite_line?: string };
				}>(session.origin, `/circles/${id}/join-preview`);
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
					// Отделённый круг (A9, 6.44): кто отделил и от какого.
					preview: preview.split?.invite_line || 'Вас позвали — выберите имя',
					time: '',
					pendingJoin: true
				});
			} catch {
				/* preview gone */
			}
		}
	}

	const merged = mergeStreetGroups(layouts, await listGroups());
	await settleLocalGroups(merged.drop, merged.fill);
	return { circles: sortStreetCircles(rows), groups: merged.groups };
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
	filters?: import('$lib/journal/search').SearchFilters,
	limit = 50
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
				const hits = await searchOrigin(session.origin, query, limit, filters);
				if (hits.length) results.push({ origin: session.origin, hits });
			} catch {
				failedInstanceNames.push(session.name);
			}
		})
	);
	return { results, failedInstanceNames };
}

export type { CircleColor };
