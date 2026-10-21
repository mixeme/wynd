import { formatPayDate } from '$lib/pay/pay';

export const SUBSCRIPTION_UNLIMITED_PREFIX = '9999-12-31';

export function isSubscriptionUnlimited(iso: string | null | undefined): boolean {
	if (!iso) return false;
	return iso.startsWith(SUBSCRIPTION_UNLIMITED_PREFIX);
}

export function parseCustomDays(value: string | number | null | undefined): number | null {
	if (value == null || value === '') return null;
	const n = typeof value === 'number' ? value : Number(String(value).trim());
	return Number.isInteger(n) && n > 0 ? n : null;
}

/** Short label for the «На сервере» table. */
export function subscriptionTableStatus(iso: string | null | undefined): string {
	if (!iso) return 'не было';
	if (isSubscriptionUnlimited(iso)) return 'бессрочно';
	const end = new Date(iso);
	if (Number.isNaN(end.getTime())) return iso;
	const now = new Date();
	if (end > now) return `до ${formatPayDate(iso)}`;
	return `истекла ${formatPayDate(iso)}`;
}

/** Subtitle on grant / request screens. */
export function subscriptionAdminSubtitle(iso: string | null | undefined): string {
	if (!iso) return 'подписки не было';
	if (isSubscriptionUnlimited(iso)) return 'бессрочно';
	const end = new Date(iso);
	if (Number.isNaN(end.getTime())) return iso;
	const now = new Date();
	if (end > now) return `подписка до ${formatPayDate(iso)}`;
	return `подписка истекла ${formatPayDate(iso)}`;
}

/** One-line subscription on the people card (9.5). */
export function subscriptionPeopleLine(iso: string | null | undefined): string {
	if (!iso) return 'не было';
	if (isSubscriptionUnlimited(iso)) return 'бессрочно';
	const end = new Date(iso);
	if (Number.isNaN(end.getTime())) return iso;
	const now = new Date();
	if (end > now) return `подписка до ${formatPayDate(iso)}`;
	return `подписка истекла ${formatPayDate(iso)}`;
}

export function payExtendHint(expiresAt: string | null | undefined): string {
	// Заявка и выдача без заявки говорят одно и то же (C44): откуда пойдёт срок
	// и что значит «Бессрочно». Что подписки не было, уже сказано под почтой.
	if (expiresAt && isSubscriptionUnlimited(expiresAt)) {
		return 'Сейчас бессрочно. Срок в днях заменит бессрочный доступ.';
	}
	const end = expiresAt ? new Date(expiresAt) : null;
	const running = end !== null && !Number.isNaN(end.getTime()) && end >= new Date();
	const from = running ? 'Срок добавится к текущей подписке.' : 'Срок пойдёт с сегодняшнего дня.';
	return `${from} «Бессрочно» — без срока и без напоминаний.`;
}
