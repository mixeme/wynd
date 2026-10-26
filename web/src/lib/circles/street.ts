import { apiFetch, ApiError } from '$lib/api/client';
import {
	addStreetOp,
	clearPins,
	deleteGroup,
	deleteStreetOp,
	listGroups,
	listPins,
	listStreetOps,
	putGroup,
	type GroupRecord,
	type StreetOpRecord
} from '$lib/idb/db';

/**
 * Раскладка улочки (план 46, A2): закрепления и группы кругов живут у учётки
 * на каждом сервере — второй телефон и очищенный браузер видят их те же.
 * Сервер знает только свои круги; группы всех серверов складываются здесь
 * по названию. Устройство держит лишь то, что серверам не нужно: свёрнута ли
 * группа и только что заведённую пустую группу. Изменение, которое сервер
 * ещё не принял (молчит), ждёт в очереди и показывается поверх его ответа.
 */

/** Группа одного сервера, как её отдаёт GET /circles. */
export interface ServerGroup {
	name: string;
	circle_ids: string[];
}

/** Раскладка одного сервера: когда закреплён круг (мс) и группы. */
export interface StreetLayout {
	pins: Map<string, number>;
	groups: ServerGroup[];
}

/** Группа на улочке — сложенная по названию со всех серверов. */
export interface StreetGroupView {
	name: string;
	/** Круги группы — `${origin}:${circleId}`. */
	keys: string[];
	collapsed: boolean;
}

export function streetKey(origin: string, circleId: string): string {
	return `${origin}:${circleId}`;
}

// Ключ — origin и id через последнее двоеточие: в origin двоеточия бывают
// (схема, порт), в id — нет. Свой сервер — пустой origin (`:id`).
function splitKey(key: string): { origin: string; circleId: string } | undefined {
	const sep = key.lastIndexOf(':');
	if (sep < 0) return undefined;
	return { origin: key.slice(0, sep), circleId: key.slice(sep + 1) };
}

function withoutCircle(groups: ServerGroup[], circleId: string): ServerGroup[] {
	return groups
		.map((g) => ({ name: g.name, circle_ids: g.circle_ids.filter((id) => id !== circleId) }))
		.filter((g) => g.circle_ids.length > 0);
}

/**
 * Раскладка сервера с изменениями, которые он ещё не принял, — как её
 * сделает сервер: круг — не больше чем в одной группе, опустевшая группа
 * уходит, переименование в занятое название сливает группы.
 */
export function applyStreetOps(base: StreetLayout, ops: StreetOpRecord[]): StreetLayout {
	const pins = new Map(base.pins);
	let groups = base.groups.map((g) => ({ name: g.name, circle_ids: [...g.circle_ids] }));
	for (const op of ops) {
		const name = op.name?.trim() ?? '';
		switch (op.op) {
			case 'pin':
				if (op.circle_id) pins.set(op.circle_id, op.pinned_at ?? Date.now());
				break;
			case 'unpin':
				if (op.circle_id) pins.delete(op.circle_id);
				break;
			case 'group': {
				if (!op.circle_id || !name) break;
				groups = withoutCircle(groups, op.circle_id);
				const target = groups.find((g) => g.name === name);
				if (target) target.circle_ids.push(op.circle_id);
				else groups.push({ name, circle_ids: [op.circle_id] });
				break;
			}
			case 'ungroup':
				if (op.circle_id) groups = withoutCircle(groups, op.circle_id);
				break;
			case 'rename': {
				const to = op.new_name?.trim() ?? '';
				const from = groups.find((g) => g.name === name);
				if (!from || !to || to === name) break;
				const into = groups.find((g) => g.name === to);
				if (into) {
					into.circle_ids.push(...from.circle_ids);
					groups = groups.filter((g) => g !== from);
				} else {
					from.name = to;
				}
				break;
			}
			case 'drop':
				groups = groups.filter((g) => g.name !== name);
				break;
		}
	}
	return { pins, groups };
}

/**
 * Группы улочки: со всех серверов по названию, по алфавиту. Записи устройства
 * дают свёрнутость; запись, чьей группы на серверах нет, — либо только что
 * заведённая пустая группа (показывается), либо опустевшая (уходит).
 */
export function mergeStreetGroups(
	servers: Array<{ origin: string; groups: ServerGroup[] }>,
	local: GroupRecord[]
): { groups: StreetGroupView[]; drop: GroupRecord[]; fill: GroupRecord[] } {
	const byName = new Map<string, string[]>();
	for (const server of servers) {
		for (const group of server.groups) {
			const keys = byName.get(group.name) ?? [];
			keys.push(...group.circle_ids.map((id) => streetKey(server.origin, id)));
			byName.set(group.name, keys);
		}
	}
	const drop: GroupRecord[] = [];
	const fill: GroupRecord[] = [];
	const seen = new Map<string, GroupRecord>();
	for (const record of local) {
		if (seen.has(record.name)) {
			drop.push(record);
			continue;
		}
		if (byName.has(record.name)) {
			seen.set(record.name, record);
			if (!record.filled) fill.push(record);
		} else if (record.filled) {
			drop.push(record);
		} else {
			seen.set(record.name, record);
		}
	}
	const names = new Set([...byName.keys(), ...seen.keys()]);
	const groups = [...names]
		.sort((a, b) => a.localeCompare(b, 'ru'))
		.map((name) => ({
			name,
			keys: byName.get(name) ?? [],
			collapsed: seen.get(name)?.collapsed ?? false
		}));
	return { groups, drop, fill };
}

/** Записи устройства после слияния: опустевшие — прочь, наполненные — с отметкой. */
export async function settleLocalGroups(drop: GroupRecord[], fill: GroupRecord[]): Promise<void> {
	for (const record of drop) await deleteGroup(record.id);
	for (const record of fill) {
		await putGroup({
			id: record.id,
			name: record.name,
			circleIds: [],
			collapsed: record.collapsed,
			filled: true
		});
	}
}

function opRequest(op: StreetOpRecord): { path: string; init: RequestInit } | undefined {
	const json = (method: string, body: unknown): RequestInit => ({
		method,
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	const circle = op.circle_id ? `/circles/${encodeURIComponent(op.circle_id)}` : '';
	switch (op.op) {
		case 'pin':
			return circle
				? {
						path: `${circle}/pin`,
						init: json(
							'PUT',
							op.pinned_at ? { pinned_at: new Date(op.pinned_at).toISOString() } : {}
						)
					}
				: undefined;
		case 'unpin':
			return circle ? { path: `${circle}/pin`, init: { method: 'DELETE' } } : undefined;
		case 'group':
			return circle ? { path: `${circle}/group`, init: json('PUT', { name: op.name }) } : undefined;
		case 'ungroup':
			return circle ? { path: `${circle}/group`, init: { method: 'DELETE' } } : undefined;
		case 'rename':
			return {
				path: '/groups/rename',
				init: json('POST', { name: op.name, new_name: op.new_name })
			};
		case 'drop':
			return { path: '/groups/delete', init: json('POST', { name: op.name }) };
	}
}

/**
 * Отправляет серверу его изменения раскладки по порядку. Молчит или занят —
 * остальное ждёт следующего раза. Отказал по существу (круга больше нет,
 * название пустое) — изменение снимается: повтор ответа не изменит.
 */
export async function flushStreetOps(origin: string): Promise<void> {
	const ops = (await listStreetOps()).filter((op) => op.origin === origin);
	for (const op of ops) {
		const request = opRequest(op);
		if (request) {
			try {
				await apiFetch(origin, request.path, request.init);
			} catch (err) {
				const refused = err instanceof ApiError && err.status < 500 && err.status !== 429;
				if (!refused) return;
			}
		}
		await deleteStreetOp(op.id);
	}
}

export async function pendingStreetOps(origin: string): Promise<StreetOpRecord[]> {
	return (await listStreetOps()).filter((op) => op.origin === origin);
}

export async function queueStreetOp(op: StreetOpRecord): Promise<void> {
	await addStreetOp(op);
}

/**
 * Раскладка, заведённая на устройстве до A2, уходит на серверы — один раз:
 * закрепления (со своим временем) и круги групп становятся изменениями в
 * очереди, на устройстве остаются названия групп со свёрнутостью. Круги
 * серверов, где человек уже не вошёл, отбрасываются.
 */
export async function moveDeviceStreetToServers(origins: string[]): Promise<void> {
	const known = new Set(origins);
	const pins = await listPins();
	if (pins.length) {
		for (const pin of pins) {
			if (known.has(pin.origin)) {
				await addStreetOp({
					origin: pin.origin,
					op: 'pin',
					circle_id: pin.circleId,
					pinned_at: pin.pinned_at
				});
			}
		}
		await clearPins();
	}
	for (const group of await listGroups()) {
		if (!group.circleIds.length) continue;
		for (const key of group.circleIds) {
			const parts = splitKey(key);
			if (parts && known.has(parts.origin)) {
				await addStreetOp({
					origin: parts.origin,
					op: 'group',
					circle_id: parts.circleId,
					name: group.name
				});
			}
		}
		await putGroup({
			id: group.id,
			name: group.name,
			circleIds: [],
			collapsed: group.collapsed,
			filled: true
		});
	}
}

/**
 * То же изменение — сразу на экране, не дожидаясь серверов: молчащий сервер
 * отвечал бы секундами, а раньше раскладка жила на устройстве и менялась
 * мгновенно. Опустевшая группа уходит, как уйдёт и после ответа.
 */
export function applyStreetOpToView(
	groups: StreetGroupView[],
	op: StreetOpRecord
): StreetGroupView[] {
	const key = op.circle_id ? streetKey(op.origin, op.circle_id) : '';
	const name = op.name?.trim() ?? '';
	const without = (list: StreetGroupView[]) =>
		list
			.map((g) => (g.keys.includes(key) ? { ...g, keys: g.keys.filter((k) => k !== key) } : g))
			.filter(
				(g) => g.keys.length > 0 || !groups.find((o) => o.name === g.name)?.keys.includes(key)
			);
	let next = groups;
	switch (op.op) {
		case 'group':
			if (!key || !name) return groups;
			next = without(groups);
			next = next.some((g) => g.name === name)
				? next.map((g) => (g.name === name ? { ...g, keys: [...g.keys, key] } : g))
				: [...next, { name, keys: [key], collapsed: false }];
			break;
		case 'ungroup':
			if (!key) return groups;
			next = without(groups);
			break;
		case 'rename': {
			const to = op.new_name?.trim() ?? '';
			const from = groups.find((g) => g.name === name);
			if (!from || !to || to === name) return groups;
			const into = groups.find((g) => g.name === to);
			next = into
				? groups
						.filter((g) => g !== from)
						.map((g) => (g === into ? { ...g, keys: [...g.keys, ...from.keys] } : g))
				: groups.map((g) => (g === from ? { ...g, name: to } : g));
			break;
		}
		case 'drop':
			next = groups.filter((g) => g.name !== name);
			break;
		default:
			return groups;
	}
	return [...next].sort((a, b) => a.name.localeCompare(b.name, 'ru'));
}

/** Серверы, где лежат круги группы, — им и отправлять переименование и удаление. */
export function groupOrigins(group: StreetGroupView): string[] {
	const origins = new Set<string>();
	for (const key of group.keys) {
		const parts = splitKey(key);
		if (parts) origins.add(parts.origin);
	}
	return [...origins];
}
