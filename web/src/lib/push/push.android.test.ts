import { afterEach, expect, it, vi } from 'vitest';
import { setAndroidPushEnabled, unsubscribePush } from './push';
import type { SessionRecord } from '$lib/idb/db';

const fixture = vi.hoisted(() => {
	const sessions = [
		{
			origin: '',
			account_id: 'first',
			token: 'first-test',
			name: 'Первый',
			email: 'first@example.invalid'
		},
		{
			origin: 'https://second.example.invalid',
			account_id: 'second',
			token: 'second-test',
			name: 'Второй',
			email: 'second@example.invalid'
		}
	];
	return { sessions };
});

vi.mock('$lib/idb/db', () => ({
	listSessions: async () => fixture.sessions,
	getSession: async (origin: string) =>
		fixture.sessions.find((session) => session.origin === origin)
}));
vi.mock('$lib/api/client', () => ({
	apiJson: async (origin: string) => ({
		vapid_public_key: origin ? 'vapid-second' : 'vapid-first'
	}),
	apiPath: (origin: string, path: string) => origin + '/api/v1' + path
}));

const endpoint = (account: string) => 'https://push.example.invalid/' + account;

function bridge() {
	const commands: Array<{
		op: string;
		account?: string;
		origin?: string;
		vapid?: string;
		endpoint?: string;
	}> = [];
	const exchanges: Array<{
		method: string;
		path: string;
		token: string;
		body: { endpoint: string };
	}> = [];
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(raw: string) {
			const request = JSON.parse(raw);
			commands.push(request);
			const result =
				request.op === 'enable' || request.op === 'status'
					? { enabled: true, allowed: true }
					: {
							subscription: {
								endpoint: endpoint(request.account),
								p256dh: request.account,
								auth: 'key'
							},
							published: request.account === 'first' ? endpoint('expired') : ''
						};
			target.onmessage?.({ data: JSON.stringify({ id: request.id, result }) });
		}
	};
	(window as typeof window & { __wyndPush?: unknown }).__wyndPush = target;
	vi.stubGlobal(
		'fetch',
		vi.fn(
			async (
				path: string,
				init: { method: string; headers: { Authorization: string }; body: string }
			) => {
				exchanges.push({
					path,
					method: init.method,
					token: init.headers.Authorization,
					body: JSON.parse(init.body)
				});
				return new Response('{}');
			}
		)
	);
	return { commands, exchanges };
}

afterEach(() => {
	delete (window as typeof window & { __wyndPush?: unknown }).__wyndPush;
	vi.unstubAllGlobals();
});

it('регистрирует два входа с ключами своих серверов и заменяет прежний адрес', async () => {
	const { commands, exchanges } = bridge();
	await setAndroidPushEnabled(true);
	expect(commands.filter((command) => command.op === 'ensure')).toMatchObject([
		{ origin: window.location.origin, account: 'first', vapid: 'vapid-first' },
		{ origin: 'https://second.example.invalid', account: 'second', vapid: 'vapid-second' }
	]);
	for (const session of fixture.sessions as SessionRecord[]) {
		expect(exchanges).toContainEqual({
			path: session.origin + '/api/v1/push/subscribe',
			method: 'POST',
			token: 'Bearer ' + session.token,
			body: { endpoint: endpoint(session.account_id), p256dh: session.account_id, auth: 'key' }
		});
	}
	expect(exchanges).toContainEqual({
		path: '/api/v1/push/subscribe',
		method: 'DELETE',
		token: 'Bearer first-test',
		body: { endpoint: endpoint('expired') }
	});
	expect(commands.filter((command) => command.op === 'published')).toHaveLength(2);
});

it('при выходе снимает регистрацию телефона и адрес этой учётки на её сервере', async () => {
	const { commands, exchanges } = bridge();
	await unsubscribePush('https://second.example.invalid');
	expect(commands).toMatchObject([{ op: 'remove', account: 'second' }]);
	expect(exchanges).toEqual([
		{
			path: 'https://second.example.invalid/api/v1/push/subscribe',
			method: 'DELETE',
			token: 'Bearer second-test',
			body: { endpoint: endpoint('second') }
		}
	]);
});
