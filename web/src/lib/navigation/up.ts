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

// Шаг назад сделал сам goUp — его не перехватываем.
let ownBack = false;

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

// Стек: экран, поверх него лист — системная «Назад» сперва закрывает лист.
const handlers: Array<() => void> = [];
const top = () => handlers[handlers.length - 1];

/** Экран с «Назад» в шапке или открытый лист: тот же обработчик и для системной кнопки. */
export function setBackHandler(fn: () => void): () => void {
	handlers.push(fn);
	tryArm();
	return () => {
		const i = handlers.lastIndexOf(fn);
		if (i >= 0) handlers.splice(i, 1);
	};
}

/**
 * Перед переходом по истории (корневой макет, beforeNavigate). true — шаг
 * отменить: вместо него позван обработчик экрана.
 */
export function interceptHistoryBack(delta: number | undefined): boolean {
	if (ownBack) {
		ownBack = false;
		return false;
	}
	const fn = top();
	if (!fn || (delta ?? 0) >= 0) return false;
	// Отменённый шаг SvelteKit откатывает через history.go — ждём отката.
	addEventListener('popstate', () => setTimeout(fn), { once: true });
	return true;
}

// Позади нет записи (вход по пушу или ссылке, переход вверх с заменой), и
// системная «Назад» закрыла бы приложение. Ставим запись-копию: шаг назад
// с неё — к родителю. Экран регистрирует «Назад» позже входа (ждёт данных),
// поэтому запись ставится при первой регистрации на той же записи истории.
let guardIndex: number | undefined;
let pendingEnter: { index: number; url: URL } | undefined;

export function armEntryGuard(url: URL): void {
	const i = historyIndex();
	if (i === undefined || entries.has(i - 1)) return;
	pendingEnter = { index: i, url };
	tryArm();
}

function tryArm() {
	if (!pendingEnter || !top()) return;
	const { index, url } = pendingEnter;
	pendingEnter = undefined;
	if (historyIndex() !== index) return;
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
		// Не сразу: SvelteKit разбирает тот же popstate после нас и сбросил бы
		// начатый переход.
		const fn = top();
		setTimeout(() => fn?.());
		// Закрыли лист, а экран остался — позади снова пусто, нужна новая копия.
		setTimeout(() => {
			if (location.pathname + location.search === here && historyIndex() === index) {
				armEntryGuard(new URL(location.href));
			}
		}, 400);
	});
}
