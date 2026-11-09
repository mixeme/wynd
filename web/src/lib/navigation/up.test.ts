import { beforeEach, expect, it, vi } from 'vitest';

vi.mock('$app/navigation', () => ({ goto: vi.fn(async () => {}) }));

import { goto } from '$app/navigation';
import { goUpFromSpot, previousEntry, recordEntry } from './up';

// Приём записи (план 46, C13): экраны, на которые попадают мимо родителя
// (баннер в ленте, хит поиска), возвращают «Назад» на экран-источник.
// Entries живут по индексу истории — подменяем состояние истории, как его
// ведёт SvelteKit. Карта entries у модуля одна на файл тестов, поэтому
// у каждого теста свой диапазон индексов. Сам обход истории — дело
// jsdom/браузера; проверяем выбор ветки: шаг назад вместо перехода.

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

it('goUpFromSpot — вкладка круга: шаг назад по истории', () => {
	const back = vi.spyOn(window.history, 'back');
	push('/circles/c2');
	push('/circles/c2/search?q=дом');
	push('/circles/c2/days/2026-11-09');
	void goUpFromSpot('/circles/c2', ['', 'search'], '/circles/c2/days');
	expect(back).toHaveBeenCalledTimes(1);
	expect(vi.mocked(goto)).not.toHaveBeenCalled();
	back.mockRestore();
});

it('goUpFromSpot — глобальный поиск (spot со слэшем): назад в поиск', () => {
	const back = vi.spyOn(window.history, 'back');
	push('/search?q=день');
	push('/circles/c3/days/2026-11-09');
	void goUpFromSpot('/circles/c3', ['', '/search'], '/circles/c3/days');
	expect(back).toHaveBeenCalledTimes(1);
	expect(vi.mocked(goto)).not.toHaveBeenCalled();
	back.mockRestore();
});

it('goUpFromSpot — источник не из spots: переход к родителю', async () => {
	const back = vi.spyOn(window.history, 'back');
	push('/circles/c4');
	push('/circles/c4/settings');
	push('/circles/c4/days/2026-11-09');
	await goUpFromSpot('/circles/c4', [''], '/circles/c4/days');
	expect(back).not.toHaveBeenCalled();
	expect(vi.mocked(goto)).toHaveBeenCalledWith('/circles/c4/days', { replaceState: true });
	back.mockRestore();
});

it('goUpFromSpot — нет предыдущей записи (глубокая ссылка): на родителя', async () => {
	const back = vi.spyOn(window.history, 'back');
	// У круга c5 записей ещё нет — позади пусто.
	push('/circles/c5/days/2026-11-09');
	await goUpFromSpot('/circles/c5', [''], '/circles/c5/days');
	expect(back).not.toHaveBeenCalled();
	expect(vi.mocked(goto)).toHaveBeenCalledWith('/circles/c5/days', { replaceState: true });
	back.mockRestore();
});
