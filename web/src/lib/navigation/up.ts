import { goto, pushState } from '$app/navigation';

/**
 * «Назад» — на уровень вверх (план 46, C13), и в шапке, и системной кнопкой.
 *
 * Экран сам знает родителя и передаёт его в `goUp`. Если прошлая запись
 * истории и есть родитель — шаг назад по истории (прокрутка родителя на
 * месте). Иначе — переход к родителю с заменой записи, чтобы история не
 * копила экраны, с которых ушли вверх.
 *
 * Системная «Назад» (жест Android, кнопка браузера) идёт по истории — на
 * прошлый экран, а не на родителя. Поэтому её шаг назад перехватываем и
 * зовём тот же обработчик, что у стрелки в шапке (`setBackHandler`).
 */

// Индекс записи истории у SvelteKit → путь с запросом. Живёт в sessionStorage:
// после перезагрузки вкладки история браузера та же.
const STORE = 'wynd:history';
let entries = new Map<number, string>();
try {
	entries = new Map(JSON.parse(sessionStorage.getItem(STORE) ?? '[]'));
} catch {
	/* нет хранилища — живём в памяти */
}

function save() {
	try {
		sessionStorage.setItem(STORE, JSON.stringify([...entries].slice(-100)));
	} catch {
		/* не страшно */
	}
}

function historyIndex(): number | undefined {
	const v = (history.state as Record<string, unknown> | null)?.['sveltekit:history'];
	return typeof v === 'number' ? v : undefined;
}

function key(url: URL): string {
	return url.pathname + url.search;
}

/** Корневой макет зовёт после каждого перехода. */
export function recordEntry(url: URL): void {
	const i = historyIndex();
	if (i === undefined) return;
	entries.set(i, key(url));
	save();
}

/** Экран, откуда пришли на текущую запись истории (приём записи, план 46, C13). */
export function previousEntry(): string | undefined {
	const i = historyIndex();
	return i !== undefined ? entries.get(i - 1) : undefined;
}

// Шаг назад сделал сам goUp — его не перехватываем.
let ownBack = false;

/**
 * Родитель по адресу — одна таблица на всё приложение. Экраны больше не
 * передают своего родителя: иначе каждый новый экран — новая забытая строка,
 * и «Назад» уезжает не туда (C89). Таблица покрыта тестом: на каждый маршрут
 * в `web/src/routes` находится правило, новый маршрут без правила ломает тест.
 */
const UP_RULES: Array<[RegExp, string]> = [
	// Запись и её альбом.
	[/^\/circles\/([^/]+)\/posts\/([^/]+)\/album$/, '/circles/$1/posts/$2'],
	[/^\/circles\/([^/]+)\/posts\/([^/]+)$/, '/circles/$1'],
	// День и альбом дня.
	[/^\/circles\/([^/]+)\/days\/([^/]+)\/album$/, '/circles/$1/days/$2'],
	[/^\/circles\/([^/]+)\/days\/([^/]+)$/, '/circles/$1/days'],
	[/^\/circles\/([^/]+)\/days$/, '/circles/$1'],
	// Квота, архив, отделение, перенос — из настроек круга.
	[/^\/circles\/([^/]+)\/quota$/, '/circles/$1/settings'],
	[/^\/circles\/([^/]+)\/quota\/[^/]+$/, '/circles/$1/quota'],
	[/^\/circles\/([^/]+)\/archive$/, '/circles/$1/settings'],
	[/^\/circles\/([^/]+)\/split$/, '/circles/$1/settings'],
	[/^\/circles\/([^/]+)\/move$/, '/circles/$1/settings'],
	// Настройки круга и их разделы.
	[/^\/circles\/([^/]+)\/settings\/([^/]+)(\/[^/]+)*$/, '/circles/$1/settings'],
	[/^\/circles\/([^/]+)\/settings$/, '/circles/$1'],
	// Экраны круга.
	[/^\/circles\/([^/]+)\/(grid|map|responses|search)$/, '/circles/$1'],
	[/^\/circles\/([^/]+)\/compose$/, '/circles/$1'],
	[/^\/circles\/([^/]+)\/arrived$/, '/circles/$1/join'],
	[/^\/circles\/([^/]+)\/join$/, '/circles/$1'],
	[/^\/circles\/([^/]+)$/, '/circles'],
	// Новый круг — шаги формы.
	[/^\/circles\/new\/(you|server)$/, '/circles/new'],
	[/^\/circles\/new$/, '/circles'],
	[/^\/circles$/, '/'],
	// Вокруг клиента.
	[/^\/search$/, '/circles'],
	[/^\/settings\/servers\/email\/code$/, '/settings/servers/email'],
	[/^\/settings\/servers\/accept\/move$/, '/settings/servers/accept'],
	[/^\/settings\/(servers|app|news)(\/[^/]+)*$/, '/settings'],
	[/^\/settings$/, '/circles'],
	[/^\/pay\/(extend|help)$/, '/pay'],
	[/^\/pay$/, '/settings'],
	[/^\/archive\/busy$/, '/circles'],
	[/^\/admin\/([^/]+)(\/[^/]+)*$/, '/admin'],
	[/^\/admin$/, '/'],
	// Вход, приглашения, вход на сервер, сканер, перенос по ссылке.
	[/^\/(invite|join|move|auth)(\/[^/]+)*$/, '/'],
	// Служебные экраны разработки.
	[/^\/dev\//, '/circles'],
	[/^\/$/, '/']
];

/** Родитель экрана по его адресу: `/circles/x/days/y` → `/circles/x/days`. */
export function upTarget(url: URL): string {
	const path = url.pathname.replace(/\/+$/, '') || '/';
	for (const [re, parent] of UP_RULES) {
		const m = path.match(re);
		if (m) return parent.replace(/\$(\d)/g, (_, n) => m[Number(n)] ?? '');
	}
	return '/circles';
}

/**
 * «Назад» — одно правило на всё приложение (C89): туда, откуда пришли; если
 * позади нет записи (вход по ссылке, пуш, переход вверх с заменой) — на
 * родителя из таблицы. Так «Назад» не зависит от того, помнит ли экран свой
 * источник, и не уводит в список кругов из альбома, открытого в ленте.
 *
 * Шаг назад по истории — когда позади запись другого адреса: тогда прокрутка и
 * фильтры источника на месте. Запись-копия (armEntryGuard) даёт позади тот же
 * адрес — её пропускаем и идём к родителю.
 */
export function navigateBack(): Promise<void> {
	// Сверху что-то открыто — закрываем его, экран остаётся.
	const closer = topOverlay();
	if (closer) {
		closer();
		return Promise.resolve();
	}
	const here = location.pathname + location.search;
	const i = historyIndex();
	const prev = i !== undefined ? entries.get(i - 1) : undefined;
	if (prev && prev !== here) {
		ownBack = true;
		history.back();
		return Promise.resolve();
	}
	const parent = upTarget(new URL(location.href));
	if (key(new URL(parent, location.origin)) === here) return Promise.resolve();
	return goto(parent, { replaceState: true });
}

export function goUp(parent: string): Promise<void> {
	const i = historyIndex();
	const target = key(new URL(parent, location.origin));
	if (i !== undefined && entries.get(i - 1) === target) {
		ownBack = true;
		history.back();
		return Promise.resolve();
	}
	return goto(parent, { replaceState: true });
}

// Оверлеи поверх экрана (листы, диалоги, просмотр фото): «Назад» — и стрелка
// в шапке, и системная — сперва закрывает верхний. Отдельный стек, а не общий
// с экранами: иначе стрелка не знает, что над экраном что-то открыто.
const overlays: Array<() => void> = [];

/** Оверлей открыт: «Назад» закроет его, а не уйдёт с экрана. */
export function setOverlayCloser(fn: () => void): () => void {
	overlays.push(fn);
	return () => {
		const i = overlays.lastIndexOf(fn);
		if (i >= 0) overlays.splice(i, 1);
	};
}

/** Верхний оверлей, если он есть. */
function topOverlay(): (() => void) | undefined {
	return overlays[overlays.length - 1];
}

/**
 * Перед переходом по истории (корневой макет, beforeNavigate). Шаг отменяем
 * только когда сверху открыт оверлей: его и закрываем. В остальных случаях шаг
 * идёт своим ходом — «назад туда, откуда пришли» и есть правило (navigateBack).
 * Отменять шаг ради обработчика экрана — гонка с откатом истории: системная
 * «Назад» уезжала на запись дальше (C89).
 */
export function interceptHistoryBack(delta: number | undefined): boolean {
	if (ownBack) {
		ownBack = false;
		return false;
	}
	if ((delta ?? 0) >= 0) return false;
	const closer = topOverlay();
	if (!closer) return false;
	addEventListener('popstate', () => setTimeout(closer), { once: true });
	return true;
}

// Позади нет записи (вход по пушу или ссылке, переход вверх с заменой), и
// системная «Назад» закрыла бы приложение. Ставим запись-копию: шаг назад
// с неё — к родителю. Экран регистрирует «Назад» позже входа (ждёт данных),
// поэтому запись ставится при первой регистрации на той же записи истории.
let guardIndex: number | undefined;

export function armEntryGuard(url: URL): void {
	const i = historyIndex();
	if (i === undefined || entries.has(i - 1)) return;
	pushState('', {});
	guardIndex = historyIndex();
	recordEntry(url);
}

if (typeof window !== 'undefined') {
	addEventListener('popstate', () => {
		if (guardIndex === undefined || historyIndex() !== guardIndex - 1) return;
		guardIndex = undefined;
		const here = location.pathname + location.search;
		const index = historyIndex();
		// Шаг назад упёрся в запись-копию: позади ничего нет — идём к родителю,
		// а не закрываем приложение.
		setTimeout(() => void navigateBack());
		// Экран остался на месте — позади снова пусто, нужна новая копия.
		setTimeout(() => {
			if (location.pathname + location.search === here && historyIndex() === index) {
				armEntryGuard(new URL(location.href));
			}
		}, 400);
	});
}
