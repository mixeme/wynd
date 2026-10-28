import { describe, expect, it } from 'vitest';
import type { SessionRecord } from '$lib/idb/db';
import { isServerClosed, serverParam, sessionForParam, setClosedServers } from './closed.svelte';

const s = (origin: string) => ({ origin, name: origin, email: 'a@example.com' }) as SessionRecord;

// C67: шлюз решает по серверу, а не по первому из списка.
describe('closed servers', () => {
	it('закрыт только названный сервер, свой — по пустому origin', () => {
		setClosedServers(['https://dacha.example.ru/']);
		expect(isServerClosed('https://dacha.example.ru')).toBe(true);
		expect(isServerClosed('https://home.example.org')).toBe(false);
		setClosedServers(['']);
		expect(isServerClosed(window.location.origin)).toBe(true);
		setClosedServers([]);
	});

	it('сервер из адреса: без параметра — первый, неизвестный — никакой', () => {
		const list = [s(''), s('https://dacha.example.ru')];
		expect(sessionForParam(list, null)).toBe(list[0]);
		expect(sessionForParam(list, 'https://dacha.example.ru')).toBe(list[1]);
		expect(sessionForParam(list, window.location.origin)).toBe(list[0]);
		expect(sessionForParam(list, 'https://other.example.net')).toBeUndefined();
		expect(decodeURIComponent(serverParam(''))).toBe(window.location.origin);
	});
});
