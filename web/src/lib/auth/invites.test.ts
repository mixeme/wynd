import { describe, expect, it } from 'vitest';
import {
	inviteCardPreview,
	isCircleInvitePeek,
	memberSubtitle,
	serverInviteSubtitle,
	type InvitePeek
} from './invites';

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

describe('memberSubtitle', () => {
	it('владелец, который пригласил, — обе части, глагол по роду', () => {
		expect(memberSubtitle({ name: 'Аня', is_owner: true, is_inviter: true, gender: 'f' })).toBe(
			'владелец · пригласила вас'
		);
		expect(memberSubtitle({ name: 'Боб', is_owner: false, is_inviter: true, gender: 'm' })).toBe(
			'пригласил вас'
		);
		expect(memberSubtitle({ name: 'Кот', is_owner: false, is_inviter: true })).toBe(
			'автор приглашения'
		);
		expect(memberSubtitle({ name: 'Аня', is_owner: true, is_inviter: false })).toBe('владелец');
	});
});

describe('inviteCardPreview', () => {
	it('глагол по роду пригласившего, без рода — без глагола', () => {
		const peek = (gender?: 'm' | 'f') =>
			({
				member_count: 2,
				members: [{ name: 'Аня', is_owner: true, is_inviter: true, gender }]
			}) as InvitePeek;
		expect(inviteCardPreview(peek('f'))).toBe('2 участника · пригласила Аня');
		expect(inviteCardPreview(peek('m'))).toBe('2 участника · пригласил Аня');
		expect(inviteCardPreview(peek())).toBe('2 участника · приглашение: Аня');
	});
});
