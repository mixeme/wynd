import { beforeEach, expect, it, vi } from 'vitest';

vi.mock('$app/navigation', () => ({ goto: vi.fn(async () => {}) }));

import { goto } from '$app/navigation';
import { goUpFromSpot, previousEntry, recordEntry } from './up';

// Приём записи (план 46, C13): экраны, на которые попадают мимо родителя
// (баннер в ленте, хит поиска), возвращают «Назад» на экран-источник.
// Entries живут по индексу истории — подменяем состояние истории, как его
// ведёт SvelteKit. Карта entries у модуля одна на файл тестов, поэтому
// у каждого теста свой диапазон индексов.

let idx = 0;
const push = (path: string, record = true): void => {
	window.history.pushState({ 'sveltekit:history': ++idx }, '', path);
	if (record) recordEntry(new URL(path, location.origin));
};

beforeEach(() => {
	sessionStorage.clear();
	vi.mocked(goto).mockClear();
	idx += 20;
});

it('previousEntry — экран, откуда пришли на текущую запись', () => {
	push('/circles/c1');
	push('/circles/c1/search?q=дом');
	expect(previousEntry()).toBe('/circles/c1');
});

it('goUpFromSpot — узнаваемый источник: шаг назад по истории', async () => {
	push('/circles/c2');
	push('/circles/c2/search?q=день');
	push('/circles/c2/days/2026-11-09');
	void goUpFromSpot('/circles/c2', ['', '/search'], '/circles/c2/days');
	await new Promise((r) => setTimeout(r, 20));
	expect(location.pathname).toBe('/circles/c2/search');
	expect(vi.mocked(goto)).not.toHaveBeenCalled();
});

it('goUpFromSpot — источник не из spots: на объявленного родителя', async () => {
	push('/circles/c3');
	push('/circles/c3/settings');
	push('/circles/c3/days/2026-11-09');
	await goUpFromSpot('/circles/c3', [''], '/circles/c3/days');
	expect(vi.mocked(goto)).toHaveBeenCalledWith('/circles/c3/days', { replaceState: true });
});

it('goUpFromSpot — нет предыдущей записи (глубокая ссылка): на родителя', async () => {
	// У круга c4 записей ещё нет — позади пусто.
	push('/circles/c4/days/2026-11-09');
	await goUpFromSpot('/circles/c4', ['', '/search'], '/circles/c4/days');
	expect(vi.mocked(goto)).toHaveBeenCalledWith('/circles/c4/days', { replaceState: true });
});
