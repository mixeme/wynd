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

/** One-line subscription on the people card (9.4). */
export function subscriptionPeopleLine(iso: string | null | undefined): string {
	if (!iso) return 'не было';
	if (isSubscriptionUnlimited(iso)) return 'бессрочно';
	const end = new Date(iso);
	if (Number.isNaN(end.getTime())) return iso;
	const now = new Date();
	if (end > now) return `подписка до ${formatPayDate(iso)}`;
	return `подписка истекла ${formatPayDate(iso)}`;
}

export function payExtendHint(
	expiresAt: string | null | undefined,
	kind: 'request' | 'grant'
): string {
	if (kind === 'grant') {
		return 'Заявки нет: скрин не спрашиваем. Срок кончился — считают от сегодня. «Бессрочно» — круги открыты, пока не выберете срок; напоминаний не будет.';
	}
	if (!expiresAt) return 'Подписки не было — считают от сегодня.';
	if (isSubscriptionUnlimited(expiresAt)) {
		return 'Сейчас бессрочно. Срок в днях заменит бессрочный доступ.';
	}
	const end = new Date(expiresAt);
	if (Number.isNaN(end.getTime()) || end < new Date()) {
		return 'Подписка кончилась — считают от сегодня.';
	}
	return 'Считают от конца текущей подписки, если она ещё идёт.';
}
