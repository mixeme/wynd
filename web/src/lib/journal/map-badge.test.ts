import { describe, expect, it } from 'vitest';
import { postsWithPlace } from './map-badge';

describe('postsWithPlace (C111)', () => {
	it('две точки одной записи — одна запись', () => {
		expect(postsWithPlace([{ post_id: 'a' }, { post_id: 'a' }, { post_id: 'b' }])).toBe(2);
	});
	it('пустой список — ноль', () => {
		expect(postsWithPlace([])).toBe(0);
	});
});
