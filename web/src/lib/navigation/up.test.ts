import { describe, expect, it, vi, beforeEach } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(async () => {}),
	pushState: vi.fn()
}));

import { goto, pushState } from '$app/navigation';
import { replaceSearchUrl } from '$lib/journal/search';
import {
	androidBackClosesApp,
	armBackGuard,
	armEntryGuard,
	armStreetGuard,
	blockedHomeBack,
	dismissOverlay,
	interceptHistoryBack,
	navigateBack,
	previousEntry,
	recordEntry,
	setOverlayCloser,
	upTarget
} from './up';

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
	vi.mocked(pushState).mockClear();
	idx += 20;
	// ownBack снимается настоящим popstate; history.back в тестах замокан,
	// поэтому после шага прошлого теста флаг остался бы висеть.
	interceptHistoryBack(1);
});

describe('upTarget — родитель по адресу', () => {
	it('native Back consumes only the top overlay and reports whether it handled the action', () => {
		const bottom = vi.fn(),
			top = vi.fn();
		const dropBottom = setOverlayCloser(bottom),
			dropTop = setOverlayCloser(top);
		expect(dismissOverlay()).toBe(true);
		expect(top).toHaveBeenCalledTimes(1);
		expect(bottom).not.toHaveBeenCalled();
		dropTop();
		expect(dismissOverlay()).toBe(true);
		expect(bottom).toHaveBeenCalledTimes(1);
		dropBottom();
		expect(dismissOverlay()).toBe(false);
	});
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
	it('C110: внешняя страница остаётся позади после замены приглашения формой имени', async () => {
		const navigation = vi
			.spyOn(performance, 'getEntriesByType')
			.mockReturnValue([{ type: 'navigate' } as PerformanceNavigationTiming]);
		const back = vi.spyOn(history, 'back').mockImplementation(() => {});
		vi.mocked(pushState).mockClear();
		try {
			push('/invite/link', false);
			recordEntry(new URL(location.href), true);
			armEntryGuard(new URL(location.href));
			history.replaceState(history.state, '', '/circles/invited/join');
			recordEntry(new URL(location.href));
			armEntryGuard(new URL(location.href));
			await navigateBack();
			expect(pushState).not.toHaveBeenCalled();
			expect(back).toHaveBeenCalledTimes(1);
			expect(goto).not.toHaveBeenCalled();
		} finally {
			back.mockRestore();
			navigation.mockRestore();
		}
	});

	it.each([
		{ type: 'navigate', length: 1 },
		{ type: 'back_forward', length: 2 },
		{ type: 'reload', length: 2 }
	])(
		'C110: без предыдущей записи остаётся переход к родителю ($type, $length)',
		async ({ type, length }) => {
			const navigation = vi
				.spyOn(performance, 'getEntriesByType')
				.mockReturnValue([{ type } as PerformanceNavigationTiming]);
			const historyLength = vi.spyOn(history, 'length', 'get').mockReturnValue(length);
			const back = vi.spyOn(history, 'back').mockImplementation(() => {});
			try {
				push('/invite/link', false);
				recordEntry(new URL(location.href), true);
				history.replaceState(history.state, '', '/circles/invited/join');
				recordEntry(new URL(location.href));
				await navigateBack();
				expect(back).not.toHaveBeenCalled();
				expect(goto).toHaveBeenCalledWith('/circles/invited', { replaceState: true });
			} finally {
				back.mockRestore();
				historyLength.mockRestore();
				navigation.mockRestore();
			}
		}
	);

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

// C154: на улочке «Назад» уходил на `/` — экран входа от запуска. При
// сохранённом входе `/` сразу уводит обратно на улочку: человек видел мигание
// загрузки. Выше улочки экранов нет — шаг не делаем.
describe('C154 — «Назад» с улочки', () => {
	it('за улочкой запись входа — остаёмся, история не двигается', () => {
		const back = vi.spyOn(history, 'back');
		push('/');
		push('/circles');
		void navigateBack();
		expect(back).not.toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
		back.mockRestore();
	});

	it('позади только запись-копия улочки — к родителю не идём', async () => {
		const back = vi.spyOn(history, 'back');
		push('/circles');
		push('/circles');
		await navigateBack();
		expect(back).not.toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
		back.mockRestore();
	});

	it('настоящий прошлый экран — шаг по истории, как раньше', () => {
		const back = vi.spyOn(history, 'back');
		push('/settings');
		push('/circles');
		void navigateBack();
		expect(back).toHaveBeenCalledTimes(1);
		expect(goto).not.toHaveBeenCalled();
		back.mockRestore();
	});

	it('шлюз «Доступ закрыт» (?closed=) возвращает на улицу', () => {
		const back = vi.spyOn(history, 'back');
		push('/circles');
		push('/circles?closed=x');
		void navigateBack();
		expect(back).toHaveBeenCalledTimes(1);
		back.mockRestore();
	});

	it('системная «Назад» с улочки к входу отменяется', () => {
		expect(interceptHistoryBack(-1, new URL('https://w/circles'), new URL('https://w/'))).toBe(
			true
		);
	});

	it('системная «Назад» с улочки на настоящий экран идёт', () => {
		expect(
			interceptHistoryBack(-1, new URL('https://w/circles'), new URL('https://w/settings'))
		).toBe(false);
	});

	it('внутри приложения «Назад» на вход не блокируется', () => {
		expect(
			interceptHistoryBack(-1, new URL('https://w/circles/c1/join'), new URL('https://w/'))
		).toBe(false);
	});

	it('сверху оверлей — он важнее правила улочки', () => {
		const closer = vi.fn();
		const drop = setOverlayCloser(closer);
		expect(interceptHistoryBack(-1, new URL('https://w/circles'), new URL('https://w/'))).toBe(
			true
		);
		drop();
	});
});

describe('blockedHomeBack — шаг с улочки к входу', () => {
	it('улочка → вход и улочка → своя запись-копия блокируются', () => {
		expect(blockedHomeBack(new URL('https://w/circles'), new URL('https://w/'))).toBe(true);
		expect(blockedHomeBack(new URL('https://w/circles'), '/')).toBe(true);
		expect(blockedHomeBack(new URL('https://w/circles?closed=x'), new URL('https://w/'))).toBe(
			true
		);
		expect(blockedHomeBack(new URL('https://w/circles'), '/circles')).toBe(true);
		expect(blockedHomeBack(new URL('https://w/circles?closed=x'), '/circles?closed=x')).toBe(true);
	});

	it('настоящие экраны, шлюз оплаты и другие точки старта не блокируются', () => {
		expect(blockedHomeBack(new URL('https://w/circles'), '/settings')).toBe(false);
		expect(blockedHomeBack(new URL('https://w/circles/c1'), '/circles')).toBe(false);
		expect(blockedHomeBack(new URL('https://w/join'), '/')).toBe(false);
		// «Доступ закрыт» — та же улица с ?closed=: это прошлый экран, шаг идёт.
		expect(blockedHomeBack(new URL('https://w/circles?closed=x'), '/circles')).toBe(false);
		expect(blockedHomeBack(new URL('https://w/circles'), '/circles?closed=x')).toBe(false);
	});
});

// C157: отмена шага истории (C154) не срабатывала в оболочке Firefox на
// Android: «Назад» с улочки перезагружал страницу — белый экран. Вместо
// отмены перед записью улочки стоит копия того же адреса: шаг приходит на неё
// внутри документа, и экран не подменяется.
describe('C157 — запись-копия улочки', () => {
	it('за улочкой только вход — ставим копию', () => {
		push('/');
		push('/circles');
		expect(armStreetGuard(new URL(location.href))).toBe(true);
		expect(pushState).toHaveBeenCalledTimes(1);
	});

	it('позади пусто (улица — точка входа) — копия', () => {
		window.history.pushState({ 'sveltekit:history': ++idx }, '', '/circles');
		recordEntry(new URL('/circles', location.origin));
		expect(armStreetGuard(new URL(location.href))).toBe(true);
		expect(pushState).toHaveBeenCalledTimes(1);
	});

	it('позади копия улочки — ставим новую', () => {
		push('/circles');
		push('/circles');
		expect(armStreetGuard(new URL(location.href))).toBe(true);
		expect(pushState).toHaveBeenCalledTimes(1);
	});

	it('позади настоящий экран — шаг идёт, копии нет', () => {
		push('/settings');
		push('/circles');
		expect(armStreetGuard(new URL(location.href))).toBe(false);
		expect(pushState).not.toHaveBeenCalled();
		expect(backTo()).toBe(true);
	});

	it('шлюз «Доступ закрыт» — не улица, копии нет', () => {
		push('/');
		window.history.pushState({ 'sveltekit:history': ++idx }, '', '/circles?closed=x');
		recordEntry(new URL(location.href));
		expect(armStreetGuard(new URL(location.href))).toBe(false);
		expect(pushState).not.toHaveBeenCalled();
	});

	it('вход и другие экраны копию улицы не получают', () => {
		push('/');
		expect(armBackGuard(new URL(location.href))).toBeUndefined();
		expect(pushState).not.toHaveBeenCalled();
		push('/circles/c1');
		expect(armStreetGuard(new URL(location.href))).toBe(false);
		expect(pushState).not.toHaveBeenCalled();
	});
});

// C158: в приложении для Android системная «Назад» на улице без прошлого
// экрана закрывает приложение, как принято на платформе; шаг на настоящий
// прошлый экран остаётся обычным.
describe('C158 — «Назад» с улочки закрывает приложение Android', () => {
	it('за улочкой только вход — приложение закрывается', () => {
		push('/');
		push('/circles');
		expect(androidBackClosesApp()).toBe(true);
	});

	it('позади пусто (улица — точка входа) — приложение закрывается', () => {
		window.history.pushState({ 'sveltekit:history': ++idx }, '', '/circles');
		recordEntry(new URL('/circles', location.origin));
		expect(androidBackClosesApp()).toBe(true);
	});

	it('позади запись-копия улицы — приложение закрывается', () => {
		push('/');
		push('/circles');
		expect(armStreetGuard(new URL(location.href))).toBe(true);
		expect(androidBackClosesApp()).toBe(true);
	});

	it('позади настоящий экран — обычный шаг по истории', () => {
		push('/settings');
		push('/circles');
		expect(androidBackClosesApp()).toBe(false);
	});

	it('шлюз «Доступ закрыт» и другие экраны приложение не закрывают', () => {
		push('/');
		push('/circles?closed=x');
		expect(androidBackClosesApp()).toBe(false);
		push('/circles/c1');
		expect(androidBackClosesApp()).toBe(false);
		push('/');
		expect(androidBackClosesApp()).toBe(false);
	});
});

/** Настоящий прошлый экран позади: navigateBack сделает шаг по истории. */
function backTo(): boolean {
	const back = vi.spyOn(history, 'back').mockImplementation(() => {});
	try {
		void navigateBack();
		return back.mock.calls.length === 1;
	} finally {
		back.mockRestore();
	}
}
