/**
 * Опции ссылки-приглашения — одни для админа (9.2) и участника (2.7): ссылка
 * участника не шире ссылки админа. «Без ограничений» и «без срока» — только
 * у админа; сервер хранит их отметками (internal/auth: UnlimitedUses,
 * NoExpiry) и участнику их не выдаёт.
 */

/** Сколько человек: число, одноразовая, без ограничений (админ) или своё. */
export type UsesChoice = 'single' | 5 | 10 | 'unlimited' | 'custom';
/** Срок: секунды одной из опций, без срока (админ) или своё. */
export type TtlChoice = number | 'forever' | 'custom';

export const USES_PRESETS = [5, 10] as const;

export const TTL_PRESETS = [
	{ label: '1 час', sec: 3600 },
	{ label: '72 часа', sec: 259200 },
	{ label: 'Неделя', sec: 604800 },
	{ label: 'Месяц', sec: 2592000 }
] as const;

/** Потолки «Своё» — те же, что у сервера (maxInviteUses, maxInviteTTL). */
export const CUSTOM_USES_MAX = 1000;
export const CUSTOM_DAYS_MAX = 365;

export const UNLIMITED_USES = 2147483647;

export function isUnlimitedUses(maxUses: number): boolean {
	return maxUses >= UNLIMITED_USES;
}

export function isNoExpiry(expiresAt: string): boolean {
	return expiresAt.startsWith('9999-');
}

export function clampCustomUses(n: number): number {
	return Math.max(1, Math.min(CUSTOM_USES_MAX, Math.round(Number(n) || 1)));
}

export function clampCustomDays(n: number): number {
	return Math.max(1, Math.min(CUSTOM_DAYS_MAX, Math.round(Number(n) || 1)));
}

export interface InviteRequest {
	kind: 'single' | 'multi';
	max_uses: number;
	ttl_sec: number;
	unlimited_uses?: boolean;
	no_expiry?: boolean;
}

/** Тело запроса на ссылку из выбранных чипов. */
export function inviteRequest(
	uses: UsesChoice,
	customUses: number,
	ttl: TtlChoice,
	customDays: number
): InviteRequest {
	const single = uses === 'single';
	const req: InviteRequest = {
		kind: single ? 'single' : 'multi',
		max_uses: single
			? 1
			: uses === 'custom'
				? clampCustomUses(customUses)
				: uses === 'unlimited'
					? 1
					: uses,
		ttl_sec:
			ttl === 'forever' ? TTL_PRESETS[0].sec : ttl === 'custom' ? clampCustomDays(customDays) * 86400 : ttl
	};
	if (uses === 'unlimited') req.unlimited_uses = true;
	if (ttl === 'forever') req.no_expiry = true;
	return req;
}

/** Чипы по уже выданной ссылке — чтобы экран показывал её как есть. */
export function choicesFromInvite(inv: {
	kind: string;
	max_uses: number;
	expires_at: string;
	created_at: string;
}): { uses: UsesChoice; customUses: number; ttl: TtlChoice; customDays: number } {
	let uses: UsesChoice;
	let customUses = 20;
	if (inv.kind === 'single') uses = 'single';
	else if (isUnlimitedUses(inv.max_uses)) uses = 'unlimited';
	else if ((USES_PRESETS as readonly number[]).includes(inv.max_uses)) uses = inv.max_uses as 5 | 10;
	else {
		uses = 'custom';
		customUses = inv.max_uses;
	}
	let ttl: TtlChoice;
	let customDays = 14;
	if (isNoExpiry(inv.expires_at)) ttl = 'forever';
	else {
		const lifeSec = (Date.parse(inv.expires_at) - Date.parse(inv.created_at)) / 1000;
		const preset = TTL_PRESETS.find((p) => Math.abs(p.sec - lifeSec) < 3600);
		if (preset) ttl = preset.sec;
		else {
			ttl = 'custom';
			customDays = clampCustomDays(lifeSec / 86400);
		}
	}
	return { uses, customUses, ttl, customDays };
}
