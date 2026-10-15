import { describe, expect, it } from 'vitest';
import { spaceNotice } from './space';

const base = { ownerName: 'Кот', youOwn: false, serverName: 'Дома Ани', circleId: 'c1' };

describe('spaceNotice (3.19, 3.20)', () => {
	it('место есть — полосы нет', () => {
		expect(spaceNotice({ ...base, spaceFull: '' })).toBeUndefined();
	});

	it('упёрся круг — называет владельца', () => {
		expect(spaceNotice({ ...base, spaceFull: 'circle' })).toEqual({
			first: 'В круге кончилось место — писать пока нельзя.',
			second: 'Освободить или попросить больше места может Кот.'
		});
	});

	it('у владельца вторая строка — ссылка на «Архив и очистку»', () => {
		const n = spaceNotice({ ...base, spaceFull: 'circle', youOwn: true });
		expect(n?.second).toBe('Освободить место или попросить больше места');
		expect(n?.href).toBe('/circles/c1/quota');
	});

	it('упёрся сервер — называет сервер, ссылки нет даже у владельца', () => {
		const n = spaceNotice({ ...base, spaceFull: 'server', youOwn: true });
		expect(n).toEqual({
			first: 'На сервере кончилось место — писать пока нельзя.',
			second: 'Помочь может администратор «Дома Ани».'
		});
	});
	it('во время архивации — когда место освободится, без ссылки', () => {
		const n = spaceNotice({
			...base,
			spaceFull: 'circle',
			youOwn: true,
			archiveCycle: { active: true, deadline: '2026-11-12T10:00:00Z' }
		});
		expect(n?.href).toBeUndefined();
		expect(n?.second).toMatch(/^Место освободится 12 ноября$/);
	});
});
