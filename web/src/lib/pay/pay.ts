import { apiFetch, apiJson } from '$lib/api/client';

export interface PayBanner {
	text: string;
	dismissible: boolean;
}

export interface PayStatus {
	required: boolean;
	expires_at: string | null;
	expired: boolean;
	pending: boolean;
	pending_at?: string | null;
	pending_comment?: string | null;
	pending_blob_filename?: string | null;
	requisites: string;
	has_requisites: boolean;
	banner: PayBanner | null;
	dismissed: boolean;
	reminder: boolean;
	reminder_days_left?: number | null;
	instance_name?: string;
}

export async function fetchPayStatus(origin: string): Promise<PayStatus> {
	return apiJson<PayStatus>(origin, '/pay/status');
}

export async function submitPayRequest(
	origin: string,
	body: { blob_id: string; comment?: string }
): Promise<void> {
	await apiJson(origin, '/pay/requests', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function dismissPayBanner(origin: string): Promise<void> {
	await apiJson(origin, '/pay/banner/dismiss', { method: 'POST' });
}

export function formatPayDate(iso: string): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return iso;
	return new Intl.DateTimeFormat('ru-RU', {
		day: 'numeric',
		month: 'long',
		year: 'numeric'
	}).format(d);
}

export function formatPayDateTime(iso: string): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return iso;
	const now = new Date();
	const sameDay =
		d.getFullYear() === now.getFullYear() &&
		d.getMonth() === now.getMonth() &&
		d.getDate() === now.getDate();
	const time = new Intl.DateTimeFormat('ru-RU', {
		hour: '2-digit',
		minute: '2-digit'
	}).format(d);
	if (sameDay) return `сегодня, ${time}`;
	const yesterday = new Date(now);
	yesterday.setDate(yesterday.getDate() - 1);
	if (
		d.getFullYear() === yesterday.getFullYear() &&
		d.getMonth() === yesterday.getMonth() &&
		d.getDate() === yesterday.getDate()
	) {
		return `вчера, ${time}`;
	}
	return `${formatPayDate(iso)}, ${time}`;
}

export function pluralDays(n: number): string {
	const mod10 = n % 10;
	const mod100 = n % 100;
	if (mod10 === 1 && mod100 !== 11) return `${n} день`;
	if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return `${n} дня`;
	return `${n} дней`;
}

export const PAY_GATEWAY_LOAD_ERROR = 'Не удалось проверить доступ. Обновите страницу.';
export const PAY_GATEWAY_I_PAID = 'Я оплатил';
export const PAY_GATEWAY_WAIT = 'Ждём ответа';

export function isPayGatewayBlocked(status: PayStatus | undefined): boolean {
	return Boolean(status?.required && status.expired && status.has_requisites);
}

export function isPayGatewayPending(status: PayStatus | undefined, blocked: boolean): boolean {
	return blocked && Boolean(status?.pending);
}

export function payGatewayExpiredHint(status: PayStatus): string {
	const name = status.instance_name || 'сервер';
	if (status.expires_at) {
		return `Подписка на «${name}» закончилась ${formatPayDate(status.expires_at)}. Круги на этом сервере не открываются, пока администратор не подтвердит оплату.`;
	}
	return `Подписки на «${name}» ещё не было. Круги на этом сервере не открываются, пока администратор не подтвердит оплату.`;
}

export function payGatewayPendingHint(status: PayStatus): string {
	const when = status.pending_at ? formatPayDate(status.pending_at) : '';
	return `Заявку отправили ${when}. Пока администратор не ответит, круги закрыты и новую заявку отправить нельзя.`;
}

export function payGatewayPendingSubtitle(status: PayStatus): string {
	const file = status.pending_blob_filename || 'скриншот';
	const comment = status.pending_comment
		? ` · ${status.pending_comment}`
		: ' · без комментария можно было';
	return `${file}${comment}`;
}
