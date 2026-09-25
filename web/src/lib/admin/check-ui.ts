import type { CheckResult } from '$lib/admin/admin';

const PROXY_IDS = new Set([
	'proxy_client',
	'proxy_body_limit',
	'proxy_headers',
	'proxy_sse',
	'proxy_timeout'
]);

function failedChecks(checks: CheckResult[]): CheckResult[] {
	return checks.filter((c) => c.status === 'fail');
}

function pluralChecks(n: number): string {
	if (n === 1) return 'Одна проверка не прошла';
	if (n === 2) return 'Две проверки не прошли';
	if (n === 3) return 'Три проверки не прошли';
	if (n === 4) return 'Четыре проверки не прошли';
	return `${n} проверок не прошли`;
}

export function checkHeadline(checks: CheckResult[]): string {
	const failed = failedChecks(checks);
	if (failed.length === 0) return 'Проверки прошли';
	return pluralChecks(failed.length);
}

export function checkSubtitle(checks: CheckResult[]): string {
	const failed = failedChecks(checks);
	if (failed.length === 0) {
		return 'Статус — формой, а не цветом. Снаружи проверяет этот браузер.';
	}
	const proxyFails = failed.filter((c) => PROXY_IDS.has(c.id));
	if (failed.length === 2 && proxyFails.length === 2) {
		return 'Обе про прокси и обе чинятся одной вставкой в конфиг. Остальное в порядке.';
	}
	if (proxyFails.length > 0 && proxyFails.length === failed.length) {
		return 'Все поломки про прокси и чинятся одной вставкой в конфиг.';
	}
	return 'Статус — формой, а не цветом. Снаружи проверяет этот браузер.';
}

export const CHECK_GROUPS: { label: string; ids: string[] }[] = [
	{
		label: 'Снаружи',
		ids: ['domain', 'https_outside', 'http_redirect']
	},
	{
		label: 'Сертификат',
		ids: ['cert_le', 'cert_chain']
	},
	{
		label: 'Прокси',
		ids: [
			'proxy_client',
			'proxy_body_limit',
			'proxy_headers',
			'proxy_sse',
			'proxy_timeout'
		]
	},
	{
		label: 'Почта',
		ids: ['mail']
	},
	{
		label: 'Пуши',
		ids: ['vapid_keys', 'push_test']
	},
	{
		label: 'Приложение',
		ids: ['pwa']
	},
	{
		label: 'Сервер',
		ids: ['clocks', 'disk_space', 'daily_routine', 'backup']
	}
];

export function proxyFixId(id: string): boolean {
	return PROXY_IDS.has(id);
}
