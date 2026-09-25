export const MAX_TEXT_BYTES = 32768;

export type MentionPart = { kind: 'text' | 'mention'; value: string };

export function textByteLength(s: string): number {
	return new TextEncoder().encode(s).length;
}

export function mentionQueryAt(
	body: string,
	cursor: number
): { start: number; query: string } | null {
	const before = body.slice(0, cursor);
	const at = before.lastIndexOf('@');
	if (at < 0) return null;
	if (at > 0 && !/\s/.test(before[at - 1])) return null;
	const query = before.slice(at + 1);
	if (/\s/.test(query)) return null;
	return { start: at, query };
}

export function filterMembersByMention<T extends { name: string }>(
	members: T[],
	query: string
): T[] {
	const q = query.toLowerCase();
	return members.filter((m) => m.name.toLowerCase().startsWith(q));
}

export function insertMention(body: string, start: number, cursor: number, name: string): string {
	const mention = `@${name}`;
	return body.slice(0, start) + mention + body.slice(cursor);
}

export function splitMentionBody(body: string): MentionPart[] {
	const parts: MentionPart[] = [];
	const re = /@([^\s@,.!?;:\n]+)/g;
	let last = 0;
	for (const match of body.matchAll(re)) {
		const idx = match.index ?? 0;
		if (idx > last) {
			parts.push({ kind: 'text', value: body.slice(last, idx) });
		}
		parts.push({ kind: 'mention', value: match[0] });
		last = idx + match[0].length;
	}
	if (last < body.length) {
		parts.push({ kind: 'text', value: body.slice(last) });
	}
	if (!parts.length) {
		parts.push({ kind: 'text', value: body });
	}
	return parts;
}
