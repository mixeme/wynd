import type { AdminInvite } from '$lib/admin/admin';
import { formatDeadline } from '$lib/format/time';
import type { CircleInvite } from '$lib/circles/settings';

export function shortInviteToken(token: string): string {
	if (token.length <= 9) return token;
	return `${token.slice(0, 4)}-${token.slice(-4)}`;
}

export function inviteRegistryTitle(inv: CircleInvite | AdminInvite): string {
	if (inv.kind === 'single') {
		return inv.uses > 0 ? 'Одноразовая · использована' : 'Одноразовая · не использована';
	}
	return `Многоразовая · ${inv.uses} из ${inv.max_uses}`;
}

export function inviteRegistrySubtitle(inv: CircleInvite | AdminInvite): string {
	return `${formatDeadline(inv.expires_at)} · ${shortInviteToken(inv.token)}`;
}

export function isLiveInvite(inv: CircleInvite | AdminInvite): boolean {
	if ('revoked_at' in inv && inv.revoked_at) return false;
	if (inv.uses >= inv.max_uses) return false;
	return new Date(inv.expires_at) > new Date();
}
