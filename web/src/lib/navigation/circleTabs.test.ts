import { beforeEach, describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

vi.mock('$app/navigation', () => ({ goto: vi.fn(async () => {}) }));

import { goto } from '$app/navigation';
import { openCircleTab } from './circleTabs';

// Вкладки круга — виды одного экрана: переключение заменяет запись истории,
// иначе «Назад» возвращает по вкладкам, а не туда, откуда открыли круг.

beforeEach(() => {
	vi.mocked(goto).mockClear();
});

describe('openCircleTab', () => {
	it('хронология — адрес круга без хвоста', async () => {
		await openCircleTab('c1', '');
		expect(goto).toHaveBeenCalledWith('/circles/c1', { replaceState: true });
	});

	it('каждая вкладка — замена записи, а не новый шаг', async () => {
		for (const suffix of ['/days', '/grid', '/map', '/responses']) {
			vi.mocked(goto).mockClear();
			await openCircleTab('c1', suffix);
			expect(goto).toHaveBeenCalledWith(`/circles/c1${suffix}`, { replaceState: true });
		}
	});

	// Переключений два — нажатие вкладки и свайп. Оба должны идти через
	// openCircleTab: иначе одно из них снова начнёт копить вкладки в истории.
	it('нажатие и свайп вкладок идут через openCircleTab', () => {
		const read = (path: string) => readFileSync(join(process.cwd(), path), 'utf8');
		expect(read('src/lib/components/chrome/CircleBar.svelte')).toContain('openCircleTab(circleId');
		expect(read('src/lib/layouts/CircleLayout.svelte')).toContain('openCircleTab(circleId');
	});
});
