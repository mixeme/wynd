import { describe, expect, it } from 'vitest';
import { detailEntryOptions } from './detailEntry';

// План 76: смена раздела в левом списке не должна копить историю.
describe('detailEntryOptions', () => {
	it('первый раздел — новая запись, смена — замена', () => {
		expect(detailEntryOptions(false)).toBeUndefined();
		expect(detailEntryOptions(true)).toEqual({ replaceState: true });
	});
});
