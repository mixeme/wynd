import { beforeEach, describe, expect, it } from 'vitest';
import { getPlacePref, hasPlace, setPlacePref, withPlace } from './place-pref';

describe('место со снимков', () => {
	beforeEach(() => localStorage.clear());

	it('по умолчанию включено и помнится по кругу', () => {
		expect(getPlacePref('', 'c1')).toBe(true);
		setPlacePref('', 'c1', false);
		expect(getPlacePref('', 'c1')).toBe(false);
		expect(getPlacePref('', 'c2')).toBe(true);
		expect(getPlacePref('https://other.example', 'c1')).toBe(true);
		setPlacePref('', 'c1', true);
		expect(getPlacePref('', 'c1')).toBe(true);
	});

	it('выключенное место снимает координаты, остальное не трогает', () => {
		const meta = { kind: 'photo', geo_lat: 55.7, geo_lng: 37.6, captured_at: 'x' };
		expect(withPlace(meta, true)).toBe(meta);
		const off = withPlace(meta, false);
		expect(off.geo_lat).toBeUndefined();
		expect(off.geo_lng).toBeUndefined();
		expect(off.captured_at).toBe('x');
		expect(meta.geo_lat).toBe(55.7);
	});

	it('hasPlace — только когда есть обе координаты', () => {
		expect(hasPlace({ geo_lat: 1, geo_lng: 2 })).toBe(true);
		expect(hasPlace({ geo_lat: 1 })).toBe(false);
		expect(hasPlace({})).toBe(false);
	});
});
