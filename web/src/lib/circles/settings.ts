import { apiJson } from '$lib/api/client';
import { invalidateSnapshots } from '$lib/idb/db';
import { fetchCircleDetail } from '$lib/journal/read-cursor';
import type { CircleDetail } from '$lib/journal/types';
import { setCircleIdentity } from '$lib/circles/meta';
import type { CircleColor } from '$lib/theme/colors';

export interface CircleSettings extends CircleDetail {
	edit_window_sec?: number | null;
	is_owner: boolean;
	can_settings: boolean;
	identity_id: string;
	identity_name: string;
	avatar_blob_id?: string;
	invite_who?: 'all' | 'owner';
	invite_kind_default?: 'single' | 'multi';
}

export interface MemberInfo {
	account_id: string;
	identity_id: string;
	name: string;
	status: 'active' | 'left_with_access' | 'gone';
	can_settings: boolean;
	is_owner: boolean;
	joined_at: string;
	can_read: boolean;
	can_write: boolean;
}

export interface IdentityNameRow {
	name: string;
	effective_at: string;
}

export interface NotifyPrefs {
	posts: boolean;
	comments_mine: boolean;
	comments_all: boolean;
	reactions: boolean;
	mentions: boolean;
	events: boolean;
	mute_until?: string | null;
}

export const QUOTA_GB = 1024 * 1024 * 1024;

export const QUOTA_CHIPS = [
	{ key: 'none', label: 'Нет', bytes: null as number | null },
	{ key: '5', label: '5 ГБ', bytes: 5 * QUOTA_GB },
	{ key: '10', label: '10 ГБ', bytes: 10 * QUOTA_GB },
	{ key: 'custom', label: 'Своё…', bytes: null as number | null }
] as const;

export type QuotaChipKey = (typeof QUOTA_CHIPS)[number]['key'];

export interface VolumeBucket {
	period: string;
	bytes: number;
	cumulative_bytes: number;
}

export interface QuotaInfo {
	used_bytes: number;
	quota_bytes?: number;
	volume: VolumeBucket[];
	freed_at_cutoff_bytes?: number;
	median_post_bytes?: number;
}

export type EditWindowKey = 'chronicle' | '10m' | '1h' | '1d' | 'unlimited' | 'custom';

export function editWindowFromSec(sec: number | null | undefined): EditWindowKey {
	if (sec === 0) return 'chronicle';
	if (sec === 600) return '10m';
	if (sec === 3600) return '1h';
	if (sec === 86400) return '1d';
	if (sec === null || sec === undefined) return 'unlimited';
	return 'custom';
}

export function customHoursFromSec(sec: number): number {
	return Math.max(1, Math.min(8760, Math.round(sec / 3600)));
}

export function editWindowToSec(key: EditWindowKey, customHours = 1): number | null {
	switch (key) {
		case 'chronicle':
			return 0;
		case '10m':
			return 600;
		case '1h':
			return 3600;
		case '1d':
			return 86400;
		case 'custom':
			return customHours * 3600;
		default:
			return null;
	}
}

export async function fetchCircleSettings(
	origin: string,
	circleId: string
): Promise<CircleSettings> {
	return apiJson<CircleSettings>(origin, `/circles/${circleId}`);
}

export async function patchCircle(
	origin: string,
	circleId: string,
	patch: {
		name?: string;
		edit_window_sec?: number | null;
		color?: CircleColor;
		invite_who?: 'all' | 'owner';
		invite_kind_default?: 'single' | 'multi';
	}
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(patch)
	});
	await invalidateSnapshots(origin, { kind: 'circles', circleId });
}

export async function fetchMembers(origin: string, circleId: string): Promise<MemberInfo[]> {
	const data = await apiJson<{ members: MemberInfo[] }>(origin, `/circles/${circleId}/members`);
	return data.members;
}

export async function renameIdentity(
	origin: string,
	circleId: string,
	name: string
): Promise<void> {
	await updateIdentity(origin, circleId, { name });
}

export async function updateIdentity(
	origin: string,
	circleId: string,
	patch: { name?: string; avatar_blob_id?: string | null }
): Promise<void> {
	const body: Record<string, string> = {};
	if (patch.name !== undefined) body.name = patch.name;
	if (patch.avatar_blob_id !== undefined) body.avatar_blob_id = patch.avatar_blob_id ?? '';
	await apiJson(origin, `/circles/${circleId}/identity`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (patch.name !== undefined) {
		await setCircleIdentity(origin, circleId, patch.name);
	}
	await invalidateSnapshots(origin, { circleId });
}

export async function setMemberCanSettings(
	origin: string,
	circleId: string,
	accountId: string,
	canSettings: boolean
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/members/${accountId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ can_settings: canSettings })
	});
	await invalidateSnapshots(origin, { circleId });
}

export async function fetchIdentityHistory(
	origin: string,
	circleId: string
): Promise<IdentityNameRow[]> {
	const data = await apiJson<{ names: IdentityNameRow[] }>(
		origin,
		`/circles/${circleId}/identity`
	);
	return data.names;
}

export interface CircleInvite {
	id: string;
	token: string;
	kind: 'single' | 'multi';
	max_uses: number;
	uses: number;
	expires_at: string;
	created_at: string;
}

export async function fetchCircleInvites(
	origin: string,
	circleId: string
): Promise<CircleInvite[]> {
	const data = await apiJson<{ invites: CircleInvite[] }>(
		origin,
		`/circles/${circleId}/invites`
	);
	return data.invites ?? [];
}

export async function revokeCircleInvite(
	origin: string,
	circleId: string,
	inviteId: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/invites/${inviteId}`, { method: 'DELETE' });
}

export async function createCircleInvite(
	origin: string,
	circleId: string,
	opts: { kind: 'single' | 'multi'; ttl_sec?: number; max_uses?: number }
): Promise<{ id: string; token: string; expires_at: string; max_uses: number }> {
	const body: Record<string, unknown> = {
		kind: opts.kind,
		max_uses: opts.max_uses ?? 1
	};
	if (opts.ttl_sec !== undefined) {
		body.ttl_sec = opts.ttl_sec;
	}
	return apiJson(origin, `/circles/${circleId}/invites`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function fetchNotifyPrefs(
	origin: string,
	circleId: string
): Promise<NotifyPrefs> {
	return apiJson<NotifyPrefs>(origin, `/circles/${circleId}/notify_prefs`);
}

export async function saveNotifyPrefs(
	origin: string,
	circleId: string,
	prefs: Partial<
		Pick<
			NotifyPrefs,
			'posts' | 'comments_mine' | 'comments_all' | 'reactions' | 'events' | 'mute_until'
		>
	>
): Promise<NotifyPrefs> {
	return apiJson<NotifyPrefs>(origin, `/circles/${circleId}/notify_prefs`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(prefs)
	});
}

export async function leaveCircle(
	origin: string,
	circleId: string,
	retainAccess: boolean
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/leave`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ retain_access: retainAccess })
	});
	await invalidateSnapshots(origin, { kind: 'circles' });
}

export async function transferOwnership(
	origin: string,
	circleId: string,
	newOwnerAccountId: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/transfer`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ new_owner_account_id: newOwnerAccountId })
	});
	await invalidateSnapshots(origin, { kind: 'circles', circleId });
}

export async function excludeMember(
	origin: string,
	circleId: string,
	accountId: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/exclude`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ account_id: accountId })
	});
	await invalidateSnapshots(origin, { circleId });
}

export async function deleteCircle(
	origin: string,
	circleId: string,
	confirmName: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}`, {
		method: 'DELETE',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name: confirmName })
	});
	await invalidateSnapshots(origin, { kind: 'circles' });
}

export async function fetchQuota(
	origin: string,
	circleId: string,
	cutoffDate?: string
): Promise<QuotaInfo> {
	const params = cutoffDate ? `?cutoff_date=${encodeURIComponent(cutoffDate)}` : '';
	return apiJson<QuotaInfo>(origin, `/circles/${circleId}/quota${params}`);
}

export async function requestQuotaExpansion(
	origin: string,
	circleId: string,
	requestedBytes: number
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/quota_requests`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ requested_bytes: requestedBytes })
	});
}

export async function startArchiveCycle(
	origin: string,
	circleId: string,
	input: { cutoff_date: string; deadline: string; reminder_before_sec: number }
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/archive`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	await invalidateSnapshots(origin, { circleId });
}

export async function moveCutoff(
	origin: string,
	circleId: string,
	cutoffDate: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/archive/cutoff`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ cutoff_date: cutoffDate })
	});
	await invalidateSnapshots(origin, { circleId });
}

export async function moveDeadline(
	origin: string,
	circleId: string,
	deadline: string
): Promise<void> {
	await apiJson(origin, `/circles/${circleId}/archive/deadline`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ deadline })
	});
	await invalidateSnapshots(origin, { circleId });
}

export async function refreshSettings(origin: string, circleId: string) {
	return fetchCircleDetail(origin, circleId);
}

export const REMINDER_OPTIONS = [
	{ label: 'За сутки', sec: 86400 },
	{ label: 'За 3 дня', sec: 3 * 86400 },
	{ label: 'За неделю', sec: 7 * 86400 }
] as const;
