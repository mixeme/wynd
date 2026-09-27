import { describe, expect, it } from 'vitest';
import { choicesFromInvite, inviteRequest, UNLIMITED_USES } from './invite-options';

describe('invite options', () => {
	it('builds requests from chips, clamping «Своё»', () => {
		expect(inviteRequest('single', 0, 3600, 0)).toEqual({ kind: 'single', max_uses: 1, ttl_sec: 3600 });
		expect(inviteRequest(10, 0, 2592000, 0)).toEqual({ kind: 'multi', max_uses: 10, ttl_sec: 2592000 });
		expect(inviteRequest('custom', 5000, 'custom', 400)).toEqual({
			kind: 'multi',
			max_uses: 1000,
			ttl_sec: 365 * 86400
		});
	});

	it('sends forever flags only for «Без ограничений» and «Без срока»', () => {
		const r = inviteRequest('unlimited', 0, 'forever', 0);
		expect(r.unlimited_uses).toBe(true);
		expect(r.no_expiry).toBe(true);
		expect(r.kind).toBe('multi');
	});

	it('maps an issued invite back to chips', () => {
		expect(
			choicesFromInvite({
				kind: 'multi',
				max_uses: UNLIMITED_USES,
				expires_at: '9999-12-31T00:00:00Z',
				created_at: '2026-09-27T10:00:00Z'
			})
		).toMatchObject({ uses: 'unlimited', ttl: 'forever' });
		expect(
			choicesFromInvite({
				kind: 'multi',
				max_uses: 37,
				expires_at: '2026-10-11T10:00:00Z',
				created_at: '2026-09-27T10:00:00Z'
			})
		).toMatchObject({ uses: 'custom', customUses: 37, ttl: 'custom', customDays: 14 });
	});
});
