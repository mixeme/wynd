import { ApiError, apiJson } from '$lib/api/client';
import type { CircleColor } from '$lib/theme/colors';

export interface InvitePeekMember {
	name: string;
	is_owner: boolean;
	is_inviter: boolean;
}

export interface InvitePeek {
	server_name: string;
	host: string;
	inviter_name?: string;
	circle_name?: string;
	color?: CircleColor;
	member_count?: number;
	members?: InvitePeekMember[];
}

export function isCircleInvitePeek(
	peek: InvitePeek
): peek is InvitePeek & {
	circle_name: string;
	color: CircleColor;
	member_count: number;
	members: InvitePeekMember[];
} {
	return Boolean(peek.circle_name) && Array.isArray(peek.members);
}

export function serverInviteSubtitle(peek: InvitePeek, fallbackHost: string): string {
	const host = peek.host || fallbackHost;
	if (peek.inviter_name) return `${host} · позвал ${peek.inviter_name}`;
	return host;
}

/** Сервер ответил отказом (4xx): ссылку отозвали, она истекла, израсходована
 *  или круг удалён. Сеть и 5xx — не то: ссылка может быть жива. */
export function isDeadInviteError(err: unknown): boolean {
	return err instanceof ApiError && err.status >= 400 && err.status < 500;
}

export async function fetchInvitePeek(origin: string, token: string): Promise<InvitePeek> {
	return apiJson<InvitePeek>(origin, `/invites/${token}`);
}

export async function claimInvite(
	origin: string,
	token: string
): Promise<{ circle_id: string; already_member: boolean }> {
	return apiJson<{ circle_id: string; already_member: boolean }>(origin, `/invites/${token}/claim`, {
		method: 'POST'
	});
}

export async function joinViaInvite(
	origin: string,
	token: string,
	input: { name: string; body?: string }
): Promise<{ circle_id: string }> {
	return apiJson<{ circle_id: string }>(origin, `/invites/${token}/join`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
}

const JOIN_TOKEN_PREFIX = 'wynd.inviteJoin.';

export function saveInviteJoinToken(circleId: string, token: string): void {
	if (typeof sessionStorage === 'undefined') return;
	sessionStorage.setItem(JOIN_TOKEN_PREFIX + circleId, token);
}

export function loadInviteJoinToken(circleId: string): string | undefined {
	if (typeof sessionStorage === 'undefined') return undefined;
	return sessionStorage.getItem(JOIN_TOKEN_PREFIX + circleId) ?? undefined;
}

export function clearInviteJoinToken(circleId: string): void {
	if (typeof sessionStorage === 'undefined') return;
	sessionStorage.removeItem(JOIN_TOKEN_PREFIX + circleId);
}

export function inviteCardPreview(peek: InvitePeek): string {
	const n = peek.member_count ?? 0;
	const count =
		n % 10 === 1 && n % 100 !== 11
			? `${n} участник`
			: n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)
				? `${n} участника`
				: `${n} участников`;
	const inviter = peek.members?.find((m) => m.is_inviter);
	if (inviter) {
		return `${count} · пригласила ${inviter.name}`;
	}
	return count;
}

export function memberAvatarColor(index: number): string {
	const palette = [
		'var(--coffee)',
		'var(--olive)',
		'var(--indigo)',
		'var(--plum)',
		'var(--teal)',
		'var(--ochre)',
		'var(--terracotta)',
		'var(--slate)'
	];
	return palette[index % palette.length];
}

export function memberSubtitle(m: InvitePeekMember): string | undefined {
	if (m.is_owner) return 'владелец';
	if (m.is_inviter) return 'пригласила вас';
	return undefined;
}
