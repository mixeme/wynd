import { describe, expect, it, vi, beforeEach } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(async () => {}),
	pushState: vi.fn()
}));

import { goto } from '$app/navigation';
import { replaceSearchUrl } from '$lib/journal/search';
import { navigateBack, previousEntry, recordEntry, setOverlayCloser, upTarget } from './up';

// «Назад» — одно правило (C89): туда, откуда пришли; если позади записи нет
// (вход по ссылке, пуш, переход вверх с заменой) — на родителя из таблицы.
// Таблица покрыта тестом маршрутов: новый экран без правила ломает проверку,
// а не «Назад» на телефоне.

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

describe('upTarget — родитель по адресу', () => {
	it('альбом записи — к записи, альбом дня — к дню', () => {
		expect(upTarget(new URL('https://w/circles/c1/posts/p1/album'))).toBe('/circles/c1/posts/p1');
		expect(upTarget(new URL('https://w/circles/c1/days/2026-11-09/album'))).toBe(
			'/circles/c1/days/2026-11-09'
		);
	});

	it('вкладки круга — к ленте круга, лента — к списку кругов', () => {
		expect(upTarget(new URL('https://w/circles/c1/grid'))).toBe('/circles/c1');
		expect(upTarget(new URL('https://w/circles/c1'))).toBe('/circles');
		expect(upTarget(new URL('https://w/circles'))).toBe('/');
	});

	it('настройки и квота — к своим разделам', () => {
		expect(upTarget(new URL('https://w/circles/c1/quota/cleanup'))).toBe('/circles/c1/quota');
		expect(upTarget(new URL('https://w/circles/c1/quota'))).toBe('/circles/c1/settings');
		expect(upTarget(new URL('https://w/circles/c1/settings/notify'))).toBe('/circles/c1/settings');
		expect(upTarget(new URL('https://w/circles/c1/settings'))).toBe('/circles/c1');
	});

	it('вход и приглашения — на вход', () => {
		expect(upTarget(new URL('https://w/auth/code'))).toBe('/');
		expect(upTarget(new URL('https://w/join?from=servers'))).toBe('/');
		expect(upTarget(new URL('https://w/invite/scan'))).toBe('/');
	});
});

// Маршруты из файловой системы: у каждого должен быть родитель, отличный от
// самого адреса. Новый экран без строки в UP_RULES провалит этот тест.
describe('таблица родителей покрывает все маршруты', () => {
	const routes: string[] = [];
	const walk = (dir: string) => {
		for (const name of readdirSync(dir)) {
			const full = join(dir, name);
			if (statSync(full).isDirectory()) walk(full);
			else if (name === '+page.svelte') routes.push(full);
		}
	};
	walk(join(process.cwd(), 'src/routes'));

	const sample = (file: string): string => {
		const path = file
			.slice(file.indexOf('src/routes') + 'src/routes'.length)
			.replace(/\\/g, '/')
			.replace(/\/\+page\.svelte$/, '');
		// Динамические сегменты — образец значения.
		return (path || '/').replace(/\[[^\]]+\]/g, 'x');
	};

	it('маршрутов нашлось больше семидесяти', () => {
		expect(routes.length).toBeGreaterThan(70);
	});

	it('каждый маршрут ведёт на другой адрес того же приложения', () => {
		const broken: string[] = [];
		for (const file of routes) {
			const path = sample(file);
			const up = upTarget(new URL(path, 'https://w'));
			// У корня выше ничего нет — это не ошибка таблицы.
			if (path === '/') continue;
			if (!up.startsWith('/') || up === path) broken.push(`${path} → ${up}`);
		}
		expect(broken).toEqual([]);
	});

	it('в файле маршрута не осталось своих родителей и списков источников', () => {
		const leftovers: string[] = [];
		for (const file of routes) {
			const text = readFileSync(file, 'utf8');
			// Экран не должен сам знать родителя: ни списков источников, ни
			// зашитого «назад» в обработчике шапки. goUp остаётся только для
			// переходов после действия (например, «Не сейчас» на входе в круг).
			const ownParent = /goUpFromSpot/.test(text) || /onback=\{\(\)[^}]*goUp\(/.test(text);
			if (ownParent) leftovers.push(file);
		}
		expect(leftovers).toEqual([]);
	});
});

describe('navigateBack', () => {
	it('есть предыдущий экран — шаг назад по истории', () => {
		const back = vi.spyOn(window.history, 'back');
		push('/circles/c2');
		push('/circles/c2/posts/p2/album');
		void navigateBack();
		expect(back).toHaveBeenCalledTimes(1);
		expect(vi.mocked(goto)).not.toHaveBeenCalled();
		back.mockRestore();
	});

	// Открыли альбом из ленты — «Назад» возвращает в ленту, а не в список
	// кругов и не закрывает приложение.
	it('альбом из ленты возвращает в ленту', () => {
		const back = vi.spyOn(window.history, 'back');
		push('/circles/c3');
		push('/circles/c3/posts/p3/album?single=1');
		void navigateBack();
		expect(back).toHaveBeenCalledTimes(1);
		expect(vi.mocked(goto)).not.toHaveBeenCalled();
		back.mockRestore();
	});

	it('позади пусто (вход по ссылке) — к родителю с заменой записи', async () => {
		const back = vi.spyOn(window.history, 'back');
		window.history.pushState({ 'sveltekit:history': ++idx }, '', '/circles/c4/days/2026-11-09');
		recordEntry(new URL('/circles/c4/days/2026-11-09', location.origin));
		await navigateBack();
		expect(back).not.toHaveBeenCalled();
		expect(vi.mocked(goto)).toHaveBeenCalledWith('/circles/c4/days', { replaceState: true });
		back.mockRestore();
	});

	// Запись-копия у входа (armEntryGuard) даёт позади тот же адрес: шаг назад
	// с неё закрыл бы приложение, поэтому идём к родителю.
	it('позади та же запись (страж входа) — к родителю', async () => {
		const back = vi.spyOn(window.history, 'back');
		push('/circles/c5/settings');
		push('/circles/c5/settings');
		await navigateBack();
		expect(back).not.toHaveBeenCalled();
		expect(vi.mocked(goto)).toHaveBeenCalledWith('/circles/c5', { replaceState: true });
		back.mockRestore();
	});

	// Лист или просмотр фото поверх экрана: «Назад» закрывает его, а не уводит
	// с экрана (C89 — «лента → альбом → назад» не должен вести в список кругов).
	it('сверху открыт лист — закрывает его, история не двигается', () => {
		const back = vi.spyOn(window.history, 'back');
		const close = vi.fn();
		const drop = setOverlayCloser(close);
		push('/circles/c7');
		push('/circles/c7/posts/p7/album?lb=3');
		void navigateBack();
		expect(close).toHaveBeenCalledTimes(1);
		expect(back).not.toHaveBeenCalled();
		expect(vi.mocked(goto)).not.toHaveBeenCalled();
		drop();
		back.mockRestore();
	});

	it('лист закрыт — следующий «Назад» уже уходит с экрана', async () => {
		const back = vi.spyOn(window.history, 'back');
		const drop = setOverlayCloser(vi.fn());
		drop();
		push('/circles/c8');
		push('/circles/c8/settings');
		await navigateBack();
		expect(back).toHaveBeenCalledTimes(1);
		back.mockRestore();
	});

	it('карта истории помнит запрос поиска (C87)', () => {
		push('/search');
		replaceSearchUrl('/search', { q: 'dom' });
		push('/circles/c6/posts/p6');
		expect(previousEntry()).toBe('/search?q=dom');
	});
});
