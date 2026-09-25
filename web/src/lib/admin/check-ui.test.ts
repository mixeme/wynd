import { describe, expect, it } from 'vitest';
import { checkHeadline, checkSubtitle } from './check-ui';
import type { CheckResult } from './admin';

function row(id: string, status: CheckResult['status']): CheckResult {
	return { id, status, title: id, detail: '' };
}

describe('checkHeadline', () => {
	it('counts only fail, not warn — as on #e9-8', () => {
		const checks = [
			row('proxy_client', 'fail'),
			row('proxy_body_limit', 'fail'),
			row('mail', 'ok'),
			row('backup', 'warn')
		];
		expect(checkHeadline(checks)).toBe('Две проверки не прошли');
		expect(checkSubtitle(checks)).toBe(
			'Обе про прокси и обе чинятся одной вставкой в конфиг. Остальное в порядке.'
		);
	});

	it('says checks passed when only warnings remain', () => {
		expect(checkHeadline([row('mail', 'warn'), row('backup', 'warn')])).toBe('Проверки прошли');
	});
});
