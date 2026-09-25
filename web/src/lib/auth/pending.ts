export type PendingAuthFlow = 'register' | 'login' | 'invite';

export interface PendingAuth {
	origin: string;
	email: string;
	flow: PendingAuthFlow;
	inviteToken?: string;
	inviteName?: string;
	circleInvite?: boolean;
	instanceName?: string;
	codeSentAt?: number;
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

export function canResendCode(pending: PendingAuth, cooldownMs = 60_000): boolean {
	if (!pending.codeSentAt) return true;
	return Date.now() - pending.codeSentAt >= cooldownMs;
}

export function resendCooldownSec(pending: PendingAuth, cooldownMs = 60_000): number {
	if (!pending.codeSentAt) return 0;
	const left = cooldownMs - (Date.now() - pending.codeSentAt);
	return left > 0 ? Math.ceil(left / 1000) : 0;
}
