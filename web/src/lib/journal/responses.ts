// «Отклики» (3.12–3.14): комментарии и реакции к уже лежащим записям,
// свежее сверху. Своих действий и правок записей тут нет, название и
// обложка дня идут в ленту строкой (wynd.html, «Правила ленты»).

import { apiJson } from '$lib/api/client';

export type ResponseKind = 'comment' | 'reaction';

export interface ResponseItem {
	kind: ResponseKind;
	seq: number;
	at: string;
	actor_id: string;
	actor_name: string;
	actor_avatar_blob_id?: string;
	comment_id?: string;
	body?: string;
	emoji?: string;
	post_id?: string;
}

export interface ResponsePostRef {
	id: string;
	author_identity_id: string;
	author_name: string;
	entry_date: string;
	created_at: string;
	excerpt: string;
	cover_blob_id?: string;
}

export interface ResponsesPage {
	items: ResponseItem[];
	posts: Record<string, ResponsePostRef>;
	read_seq: number;
	has_more: boolean;
}

export async function fetchResponses(
	origin: string,
	circleId: string,
	before = 0
): Promise<ResponsesPage> {
	const q = before > 0 ? `?before=${before}` : '';
	const page = await apiJson<ResponsesPage>(origin, `/circles/${circleId}/responses${q}`);
	return {
		items: page.items ?? [],
		posts: page.posts ?? {},
		read_seq: page.read_seq ?? 0,
		has_more: Boolean(page.has_more)
	};
}

export async function markResponsesRead(origin: string, circleId: string, seq: number): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/responses/read`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ seq })
	});
}

/** Строка 3.13. Реакции подряд на одну запись — одна строка с именами. */
export interface ResponseRow {
	key: string;
	kind: ResponseKind;
	/** Самый свежий отклик строки: по нему время, черта и место в списке. */
	lead: ResponseItem;
	/** Имена через запятую: одна реакция — одно имя, как под записью. */
	names: string[];
	/** Знак реакции, если он у всех одинаковый; иначе знак самой свежей. */
	emoji?: string;
	isNew: boolean;
}

export function groupResponses(items: ResponseItem[], readSeq: number): ResponseRow[] {
	const rows: ResponseRow[] = [];
	for (const item of items) {
		const prev = rows[rows.length - 1];
		const isNew = item.seq > readSeq;
		if (
			prev &&
			item.kind === 'reaction' &&
			prev.kind === 'reaction' &&
			prev.lead.post_id === item.post_id &&
			prev.isNew === isNew
		) {
			if (!prev.names.includes(item.actor_name)) prev.names.push(item.actor_name);
			continue;
		}
		rows.push({
			key: `${item.kind}-${item.seq}`,
			kind: item.kind,
			lead: item,
			names: [item.actor_name],
			emoji: item.emoji,
			isNew
		});
	}
	return rows;
}

/** Где кончается новое: индекс первой прочитанной строки, если выше есть новые. */
export function responsesDividerIndex(rows: ResponseRow[]): number | null {
	const first = rows.findIndex((r) => !r.isNew);
	return first > 0 ? first : null;
}

/** Подпись вида отклика — без глагола: род человека система не знает. */
export function responseKindLabel(kind: ResponseKind): string {
	switch (kind) {
		case 'comment':
			return 'комментарий';
		case 'reaction':
			return 'реакция';
	}
}

/** Куда ведёт нажатие: комментарий — к нему в обсуждении, реакция — в запись. */
export function responseHref(circleId: string, item: ResponseItem): string {
	const base = `/circles/${circleId}`;
	if (item.post_id) {
		const comment = item.kind === 'comment' && item.comment_id ? `?comment=${item.comment_id}` : '';
		return `${base}/posts/${item.post_id}${comment}`;
	}
	return base;
}
