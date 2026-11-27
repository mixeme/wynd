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
// Для приглашения с другой страницы у предыдущей записи нет пути Wynd.
type HistoryEntry = string | { path: string; previousDocument: true };
let entries = new Map<number, HistoryEntry>();
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

function hasPreviousDocument(index: number): boolean {
	const entry = entries.get(index);
	return typeof entry === 'object' && entry?.previousDocument === true;
}

/** Корневой макет зовёт после каждого перехода, включая вход по ссылке. */
export function recordEntry(url: URL, openingInvite = false): void {
	const i = historyIndex();
	if (i === undefined) return;
	// Новая загрузка дописывается в конец истории. При перезагрузке или
	// переходе по истории length может включать записи впереди.
	const fromPreviousDocument =
		openingInvite &&
		history.length > 1 &&
		(performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined)
			?.type === 'navigate';
	const path = key(url);
	// claimInvite заменяет адрес на этой же записи истории.
	entries.set(
		i,
		fromPreviousDocument || hasPreviousDocument(i) ? { path, previousDocument: true } : path
	);
	save();
}

/** Экран, откуда пришли на текущую запись истории (приём записи, план 46, C13). */
export function previousEntry(): string | undefined {
	const i = historyIndex();
	const entry = i !== undefined ? entries.get(i - 1) : undefined;
	return typeof entry === 'string' ? entry : entry?.path;
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
	[/^\/share$/, '/circles'],
	[/^\/search$/, '/circles'],
	[/^\/settings\/app\/interface$/, '/settings/app'],
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

/** Улочка — дом вошедшего приложения: выше неё только экран входа. */
const HOME = '/circles';

function barePath(target: URL | string): string {
	const path = typeof target === 'string' ? target.split(/[?#]/)[0] : target.pathname;
	return path.replace(/\/+$/, '') || '/';
}

/**
 * «Назад» с улочки к экрану входа — петля, а не возврат (C154). Запись `/`
 * остаётся в истории от запуска: экран входа увёл вошедшего на улочку. Но `/`
 * при сохранённом входе снова уводит на `/circles`, и человек видит мигание
 * загрузки. Выше улочки экранов нет, поэтому такой шаг не делаем. Назад на
 * настоящий прошлый экран (настройки, поиск, «Доступ закрыт» с `?closed=`) идёт
 * как раньше. Запись-копия улицы (armEntryGuard) — тот же адрес с тем же
 * поиском, её тоже пропускаем.
 *
 * В нативной оболочке Android этот же разбор значит больше: шаг назад с улочки
 * увёл бы из приложения, и там «Назад» закрывает его (C158,
 * androidBackClosesApp).
 */
export function blockedHomeBack(from: URL, to?: URL | string): boolean {
	if (barePath(from) !== HOME) return false;
	if (to === undefined) return true;
	const target = typeof to === 'string' ? new URL(to, location.origin) : to;
	if (barePath(target) === '/') return true;
	return target.pathname === from.pathname && target.search === from.search;
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
	if (dismissOverlay()) return Promise.resolve();
	const url = new URL(location.href);
	const here = key(url);
	const i = historyIndex();
	if (i !== undefined && hasPreviousDocument(i)) {
		history.back();
		return Promise.resolve();
	}
	const prev = previousEntry();
	if (prev && prev !== here) {
		if (!blockedHomeBack(url, prev)) {
			ownBack = true;
			history.back();
		}
		return Promise.resolve();
	}
	const parent = upTarget(url);
	if (key(new URL(parent, location.origin)) === here) return Promise.resolve();
	if (blockedHomeBack(url, parent)) return Promise.resolve();
	return goto(parent, { replaceState: true });
}

export function goUp(parent: string): Promise<void> {
	const target = key(new URL(parent, location.origin));
	if (previousEntry() === target) {
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

/** Нативная «Назад» закрывает окно до перехода между документами WebView. */
export function dismissOverlay(): boolean {
	// Снять со стека до вызова: ondismiss может сам позвать navigateBack.
	const closer = overlays.pop();
	if (!closer) return false;
	closer();
	return true;
}

/** Верхний оверлей, если он есть. */
function topOverlay(): (() => void) | undefined {
	return overlays[overlays.length - 1];
}

/**
 * Перед переходом по истории (корневой макет, beforeNavigate). Шаг отменяем
 * только когда сверху открыт оверлей: его и закрываем, — или когда он ведёт
 * с улочки на экран входа (C154). В остальных случаях шаг идёт своим ходом —
 * «назад туда, откуда пришли» и есть правило (navigateBack). Отменять шаг ради
 * обработчика экрана — гонка с откатом истории: системная «Назад» уезжала на
 * запись дальше (C89).
 */
export function interceptHistoryBack(delta: number | undefined, from?: URL, to?: URL): boolean {
	if (ownBack) {
		ownBack = false;
		return false;
	}
	if ((delta ?? 0) >= 0) return false;
	const closer = topOverlay();
	if (!closer) return Boolean(from && to && blockedHomeBack(from, to));
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
	if (i === undefined || entries.has(i - 1) || hasPreviousDocument(i)) return;
	// У экрана входа копия мешала бы: вошедшего он сразу уводит на улицу с
	// заменой записи (C157), а улица ставит свою копию. Иначе замена легла бы
	// на копию входа, а сам вход остался бы позади улицы.
	if (barePath(url) === '/') return;
	pushState('', {});
	guardIndex = historyIndex();
	recordEntry(url);
}

/**
 * Позади улицы нет настоящего экрана — шаг назад с неё остаётся на месте.
 * Тот же разбор, что в navigateBack: позади только вход (`/`), запись-копия
 * улицы с тем же адресом или ничего; внешний документ — настоящий шаг.
 */
function streetStays(url: URL): boolean {
	const i = historyIndex();
	if (i === undefined || hasPreviousDocument(i)) return false;
	const here = key(url);
	const prev = previousEntry();
	if (!prev || prev === here) return true;
	return barePath(new URL(prev, location.origin)) === '/';
}

/**
 * Системная «Назад» в нативной оболочке Android на улице, за которой нет
 * настоящего экрана: приложение закрывается, как принято на платформе (C158).
 * Возвращает true, только когда шаг действительно увёл бы из приложения, —
 * назад на прошлый экран (настройки, поиск, другой круг) идёт как раньше.
 * Браузера это не касается: страница не может закрыть саму себя, там остаётся
 * запись-копия улицы (C157).
 */
export function androidBackClosesApp(url: URL = new URL(location.href)): boolean {
	if (barePath(url) !== HOME || url.search) return false;
	return streetStays(url);
}

/**
 * Улица — дом вошедшего приложения: держим перед её записью копию того же
 * адреса (C157). «Назад» приходит на копию и обработчик ставит следующую:
 * шаг остаётся внутри документа, SvelteKit не подменяет экран и ничего не
 * загружает. Без копии шаг уходил на запись входа `/`, и там, где отмена шага
 * не срабатывает (оболочка Firefox на Android), страница перезагружалась —
 * белый экран. Копия ставится, только когда позади улицы нет настоящего
 * экрана: назад в настройки, поиск, шлюз «Доступ закрыт» или круг идёт как
 * раньше.
 */
export function armStreetGuard(url: URL): boolean {
	if (barePath(url) !== HOME || url.search) return false;
	if (!streetStays(url)) return false;
	const i = historyIndex();
	if (i === undefined) return false;
	pushState('', {});
	guardIndex = historyIndex();
	recordEntry(url);
	return true;
}

/** Копия записи, чтобы системная «Назад» не увела с текущего экрана. */
export function armBackGuard(url: URL): void {
	if (!armStreetGuard(url)) armEntryGuard(url);
}

if (typeof window !== 'undefined') {
	addEventListener('popstate', () => {
		if (guardIndex === undefined || historyIndex() !== guardIndex - 1) return;
		guardIndex = undefined;
		const here = location.pathname + location.search;
		const index = historyIndex();
		const url = new URL(location.href);
		// Шаг назад упёрся в запись-копию: позади ничего нет — идём к родителю,
		// а не закрываем приложение. На улице с пустым хвостом (C157) копию
		// ставим сразу: следующий «Назад» не должен успеть дойти до входа.
		const stays = barePath(url) === HOME && streetStays(url);
		setTimeout(() => void navigateBack());
		// Экран остался на месте — позади снова пусто, нужна новая копия.
		setTimeout(
			() => {
				if (location.pathname + location.search === here && historyIndex() === index) {
					armBackGuard(new URL(location.href));
				}
			},
			stays ? 0 : 400
		);
	});
}
