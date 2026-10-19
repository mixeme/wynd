import { describe, expect, it } from 'vitest';
import { formatPayDate, payGatewayPendingHint } from './pay';

describe('formatPayDate', () => {
	it('drops «г.» so a sentence does not end with two dots', () => {
		expect(formatPayDate('2026-10-19T12:00:00Z')).toBe('19 октября 2026');
		expect(payGatewayPendingHint({ pending_at: '2026-10-19T12:00:00Z' } as never)).toMatch(
			/^Заявку отправили 19 октября 2026\. /
		);
	});
});
