import { apiJson } from '$lib/api/client';
import { fetchInstance } from '$lib/auth/auth';
import { displayHost } from '$lib/auth/origin';
import { collectExternalReport, type ExternalReport } from '$lib/admin/external-probe';
import { getAdminSession } from '$lib/idb/db';

async function origin(): Promise<string> {
	return (await getAdminSession())?.origin ?? '';
}

async function adminJson<T>(path: string, init?: RequestInit): Promise<T> {
	return apiJson<T>(await origin(), path, init);
}

export interface AdminStorageCircle {
	id: string;
	name: string;
	color: string;
	posts: number;
	media_bytes: number;
	quota_bytes: number | null;
	quota_custom: boolean;
	owner_email: string;
}

export interface AdminStorage {
	used_bytes: number;
	quota_bytes: number;
	default_circle_quota_bytes: number | null;
	circles: AdminStorageCircle[];
}

export interface CompressionSettings {
	photo_max_px: number;
	photo_quality: number;
	video_max_height: number;
	video_bitrate_kbps: number;
	attachment_max_bytes: number;
}

export interface AccessSettings {
	name: string;
	registration_mode: 'open' | 'invite' | 'closed';
}

export interface AdminAccount {
	id: string;
	email: string;
	circle_count: number;
	created_at: string;
	blocked: boolean;
}

export interface AdminAccountDetail {
	id: string;
	email: string;
	created_at: string;
	last_login_at: string | null;
	blocked: boolean;
	owns_circle: boolean;
	circles: {
		id: string;
		name: string;
		color: string;
		role: 'owner' | 'member';
		joined_at?: string;
	}[];
}

export interface AdminInvite {
	id: string;
	token: string;
	kind: string;
	max_uses: number;
	uses: number;
	expires_at: string;
	revoked_at?: string;
	created_at: string;
}

export interface QuotaRequest {
	id: string;
	circle_id: string;
	requester_id: string;
	requester_email: string;
	requested_bytes: number;
	status: string;
	created_at: string;
}

export interface CheckResult {
	id: string;
	status: 'ok' | 'warn' | 'fail' | 'na';
	title: string;
	detail: string;
}

export interface ProxySnippet {
	kind: string;
	snippet: string;
}

export async function fetchStorage(): Promise<AdminStorage> {
	return adminJson('/admin/storage');
}

export async function setStorageQuota(quotaBytes: number): Promise<void> {
	await adminJson('/admin/storage/quota', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ quota_bytes: quotaBytes })
	});
}

export async function setDefaultCircleQuota(
	defaultCircleQuotaBytes: number | null
): Promise<void> {
	await adminJson('/admin/storage/default_quota', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ default_circle_quota_bytes: defaultCircleQuotaBytes })
	});
}

export async function setCircleQuota(
	circleId: string,
	body: { custom: boolean; quota_bytes?: number | null }
): Promise<void> {
	await adminJson(`/admin/circles/${circleId}/quota`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function fetchCompression(): Promise<CompressionSettings> {
	return adminJson('/admin/compression');
}

export async function saveCompression(body: CompressionSettings): Promise<void> {
	await adminJson('/admin/compression', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function fetchAccess(): Promise<AccessSettings> {
	return adminJson('/admin/access');
}

export async function saveAccess(body: Partial<AccessSettings>): Promise<void> {
	await adminJson('/admin/access', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function fetchAccounts(): Promise<AdminAccount[]> {
	const data = await adminJson<{ accounts: AdminAccount[] }>('/admin/accounts');
	return data.accounts ?? [];
}

export async function fetchAccount(id: string): Promise<AdminAccountDetail> {
	return adminJson(`/admin/accounts/${id}`);
}

export async function deleteAccount(id: string): Promise<void> {
	await adminJson(`/admin/accounts/${id}`, { method: 'DELETE' });
}

export async function blockAccount(id: string): Promise<void> {
	await adminJson(`/admin/accounts/${id}/block`, { method: 'POST' });
}

export async function unblockAccount(id: string): Promise<void> {
	await adminJson(`/admin/accounts/${id}/unblock`, { method: 'POST' });
}

export async function fetchInvites(): Promise<AdminInvite[]> {
	const data = await adminJson<{ invites: AdminInvite[] }>('/admin/invites');
	return data.invites ?? [];
}

export async function createServerInvite(opts: {
	kind: 'single' | 'multi';
	max_uses: number;
	ttl_sec: number;
}): Promise<{ token: string; expires_at: string; max_uses: number }> {
	return adminJson('/admin/invites', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(opts)
	});
}

export async function revokeInvite(id: string): Promise<void> {
	await adminJson(`/admin/invites/${id}`, { method: 'DELETE' });
}

export async function fetchQuotaRequests(): Promise<QuotaRequest[]> {
	const data = await adminJson<{ requests: QuotaRequest[] }>('/admin/quota_requests');
	return data.requests ?? [];
}

export async function resolveQuotaRequest(id: string, approve: boolean): Promise<void> {
	const action = approve ? 'approve' : 'reject';
	await adminJson(`/admin/quota_requests/${id}/${action}`, { method: 'POST' });
}

export async function runChecks(): Promise<CheckResult[]> {
	const origin = (await getAdminSession())?.origin ?? '';
	const instance = await fetchInstance(origin);
	const external: ExternalReport | undefined = instance.loopback
		? undefined
		: await collectExternalReport(origin);
	const data = await adminJson<{ checks: CheckResult[] }>('/admin/check', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ external })
	});
	return data.checks ?? [];
}

export async function fetchProxySnippet(kind: string): Promise<ProxySnippet> {
	return adminJson(`/admin/proxy/${kind}`);
}

export async function fetchVapidPublicKey(): Promise<string> {
	const data = await adminJson<{ public_key: string }>('/admin/push/vapid');
	return data.public_key;
}

export async function sendAdminPushTest(): Promise<void> {
	await adminJson('/admin/push/test', { method: 'POST' });
}

export async function sendSmtpTest(to: string): Promise<void> {
	await adminJson('/admin/smtp/test', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ to })
	});
}

export interface SmtpSettings {
	host: string;
	port: number;
	username: string;
	from: string;
	configured: boolean;
	test_sent_at?: string | null;
}

export async function fetchSmtp(): Promise<SmtpSettings> {
	return adminJson('/admin/smtp');
}

export async function saveSmtp(body: {
	host: string;
	port: number;
	username: string;
	password: string;
	from: string;
}): Promise<void> {
	await adminJson('/admin/smtp', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export interface PayHub {
	requisites: string;
	donate: { show: boolean; until: string };
	subscription: { required: boolean; pending_count: number };
}

export interface PayDonateSettings {
	text: string;
	show: boolean;
	dismissible: boolean;
	until: string;
}

export interface PaySubscriptionSettings {
	required: boolean;
	remind_days: number;
}

export interface PayRequest {
	id: string;
	account_id: string;
	account_email: string;
	blob_id?: string;
	blob_filename?: string;
	blob_size_bytes?: number;
	blob_deleted?: boolean;
	comment?: string | null;
	status: string;
	created_at: string;
	resolved_at?: string | null;
	subscription_expires_at?: string | null;
}

export async function fetchPayHub(): Promise<PayHub> {
	return adminJson('/admin/pay');
}

export async function savePayRequisites(requisites: string): Promise<void> {
	await adminJson('/admin/pay', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ requisites })
	});
}

export async function fetchPayDonate(): Promise<PayDonateSettings> {
	return adminJson('/admin/pay/donate');
}

export async function savePayDonate(body: PayDonateSettings): Promise<void> {
	await adminJson('/admin/pay/donate', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function fetchPaySubscription(): Promise<PaySubscriptionSettings> {
	return adminJson('/admin/pay/subscription');
}

export async function savePaySubscription(body: PaySubscriptionSettings): Promise<void> {
	await adminJson('/admin/pay/subscription', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function fetchPayRequests(): Promise<PayRequest[]> {
	const data = await adminJson<{ requests: PayRequest[] }>('/admin/pay/requests');
	return data.requests ?? [];
}

export async function fetchPayRequest(id: string): Promise<PayRequest> {
	return adminJson(`/admin/pay/requests/${id}`);
}

export async function approvePayRequest(id: string, days: number): Promise<void> {
	await adminJson(`/admin/pay/requests/${id}/approve`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ days })
	});
}

export async function rejectPayRequest(id: string): Promise<void> {
	await adminJson(`/admin/pay/requests/${id}/reject`, { method: 'POST' });
}

export async function serverCaption(): Promise<string> {
	const session = await getAdminSession();
	const host = displayHost(session?.origin ?? '');
	try {
		const access = await fetchAccess();
		return access.name ? `${access.name} · ${host}` : host;
	} catch {
		return host;
	}
}
