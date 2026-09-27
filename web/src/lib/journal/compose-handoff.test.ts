import { describe, expect, it } from 'vitest';
import { handComposePhotos, takeComposePhotos } from './compose-handoff';

const file = (name: string) => new File(['x'], name, { type: 'image/jpeg' });

describe('compose handoff', () => {
	it('отдаёт файлы своему кругу один раз', () => {
		handComposePhotos('c1', [file('a.jpg')]);
		expect(takeComposePhotos('c1').map((f) => f.name)).toEqual(['a.jpg']);
		expect(takeComposePhotos('c1')).toEqual([]);
	});

	it('чужой круг файлы не получает и очищает их', () => {
		handComposePhotos('c1', [file('a.jpg')]);
		expect(takeComposePhotos('c2')).toEqual([]);
		expect(takeComposePhotos('c1')).toEqual([]);
	});
});
