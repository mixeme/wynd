import type { AdminInvite } from '$lib/admin/admin';
import { formatDeadline } from '$lib/format/time';
import type { CircleInvite } from '$lib/circles/settings';
import { isNoExpiry, isUnlimitedUses } from '$lib/circles/invite-options';

export function shortInviteToken(token: string): string {
	if (token.length <= 9) return token;
	return `${token.slice(0, 4)}-${token.slice(-4)}`;
}

export function inviteRegistryTitle(inv: CircleInvite | AdminInvite): string {
	if (inv.kind === 'single') {
		return inv.uses > 0 ? 'Одноразовая · использована' : 'Одноразовая · не использована';
	}
	if (isUnlimitedUses(inv.max_uses)) return `Многоразовая · ${inv.uses} · без ограничений`;
	return `Многоразовая · ${inv.uses} из ${inv.max_uses}`;
}

export function inviteRegistrySubtitle(inv: CircleInvite | AdminInvite): string {
	const deadline = isNoExpiry(inv.expires_at) ? 'без срока' : formatDeadline(inv.expires_at);
	return `${deadline} · ${shortInviteToken(inv.token)}`;
}

export function isLiveInvite(inv: CircleInvite | AdminInvite): boolean {
	if ('revoked_at' in inv && inv.revoked_at) return false;
	if (inv.uses >= inv.max_uses) return false;
	return new Date(inv.expires_at) > new Date();
}
