export type PendingAuthFlow = 'register' | 'login' | 'invite';

export type CodeDelivery = 'log' | 'mail';

export interface PendingAuth {
	origin: string;
	email: string;
	flow: PendingAuthFlow;
	inviteToken?: string;
	inviteName?: string;
	circleInvite?: boolean;
	instanceName?: string;
	codeDelivery?: CodeDelivery;
	codeSentAt?: number;
	/** Earliest time (ms) when resend is allowed after rate_limited. */
	retryUntil?: number;
}

/** Когда ушёл код и когда можно просить следующий — общее у входа и смены почты. */
export interface CodeTiming {
	codeSentAt?: number;
	retryUntil?: number;
}

const KEY = 'wynd:pending-auth';

export function loadPendingAuth(): PendingAuth | undefined {
	if (typeof sessionStorage === 'undefined') return undefined;
	try {
		const raw = sessionStorage.getItem(KEY);
		if (!raw) return undefined;
		return JSON.parse(raw) as PendingAuth;
	} catch {
		return undefined;
	}
}

export function savePendingAuth(pending: PendingAuth): void {
	sessionStorage.setItem(KEY, JSON.stringify(pending));
}

export function clearPendingAuth(): void {
	sessionStorage.removeItem(KEY);
}

export function canResendCode(pending: CodeTiming, cooldownMs = 60_000): boolean {
	if (pending.retryUntil && Date.now() < pending.retryUntil) return false;
	if (!pending.codeSentAt) return true;
	return Date.now() - pending.codeSentAt >= cooldownMs;
}

export function resendCooldownSec(pending: CodeTiming, cooldownMs = 60_000): number {
	if (pending.retryUntil) {
		const left = pending.retryUntil - Date.now();
		if (left > 0) return Math.ceil(left / 1000);
	}
	if (!pending.codeSentAt) return 0;
	const left = cooldownMs - (Date.now() - pending.codeSentAt);
	return left > 0 ? Math.ceil(left / 1000) : 0;
}

export function applyRateLimitToPending<T extends CodeTiming>(
	pending: T,
	retryAfterSec: number
): T {
	return { ...pending, retryUntil: Date.now() + retryAfterSec * 1000 };
}
