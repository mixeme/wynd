import { describe, expect, it } from 'vitest';
import { isCircleInvitePeek, serverInviteSubtitle, type InvitePeek } from './invites';

const serverPeek: InvitePeek = {
	server_name: 'У Славы',
	host: 'wynd.u-slavy.ru',
	inviter_name: 'Слава'
};

const circlePeek: InvitePeek = {
	server_name: 'Дом Ани',
	host: 'home.example.org',
	circle_name: 'Семья',
	color: 'olive',
	member_count: 1,
	members: [{ name: 'Аня', is_owner: true, is_inviter: true }]
};

describe('isCircleInvitePeek', () => {
	it('accepts circle metadata', () => {
		expect(isCircleInvitePeek(circlePeek)).toBe(true);
	});

	it('rejects a server invite', () => {
		expect(isCircleInvitePeek(serverPeek)).toBe(false);
	});
});

describe('serverInviteSubtitle', () => {
	it('puts the inviter after the host', () => {
		expect(serverInviteSubtitle(serverPeek, 'fallback.example')).toBe(
			'wynd.u-slavy.ru · позвал Слава'
		);
	});

	it('keeps only the host when the name is unknown', () => {
		expect(serverInviteSubtitle({ server_name: 'Дом Ани', host: '' }, '127.0.0.1')).toBe(
			'127.0.0.1'
		);
	});
});
