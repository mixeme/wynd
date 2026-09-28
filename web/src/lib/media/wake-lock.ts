// Экран не гаснет, пока идёт сжатие или загрузка медиа (stack.html, «Видео в
// фоне»): погасший экран сворачивает вкладку, и браузер может её выгрузить
// посреди работы. Держателей несколько (сжатие на экране записи, слив
// очереди) — блокировка одна, снимается, когда уходит последний. Браузер сам
// снимает её, когда вкладка скрыта; при возврате берём заново. Без Wake Lock
// API (Firefox до 126, старые WebView) — молча ничего.

type Sentinel = { released: boolean; release(): Promise<void> };
type WakeLockApi = { request(type: 'screen'): Promise<Sentinel> };

let holders = 0;
let sentinel: Sentinel | null = null;
let pending: Promise<void> | null = null;
let listening = false;

function api(): WakeLockApi | undefined {
	if (typeof navigator === 'undefined') return undefined;
	return (navigator as Navigator & { wakeLock?: WakeLockApi }).wakeLock;
}

function acquire(): Promise<void> {
	// Запрос уже в пути — второй держатель ждёт его, а не просит вторую.
	pending ??= request().finally(() => {
		pending = null;
	});
	return pending;
}

async function request(): Promise<void> {
	const wl = api();
	if (!wl || holders === 0) return;
	if (sentinel && !sentinel.released) return;
	if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return;
	try {
		sentinel = await wl.request('screen');
		// Пока ждали ответа, последний держатель мог уйти.
		if (holders === 0) void releaseSentinel();
	} catch {
		// Отказ (энергосбережение, политика) — работаем без блокировки.
		sentinel = null;
	}
}

async function releaseSentinel(): Promise<void> {
	const s = sentinel;
	sentinel = null;
	if (s && !s.released) {
		try {
			await s.release();
		} catch {
			/* уже снята */
		}
	}
}

function onVisibility(): void {
	if (document.visibilityState === 'visible' && holders > 0) void acquire();
}

/** Держать экран включённым до вызова возвращённой функции (повтор — без эффекта). */
export function holdWakeLock(): () => void {
	if (!api()) return () => {};
	if (!listening && typeof document !== 'undefined') {
		document.addEventListener('visibilitychange', onVisibility);
		listening = true;
	}
	holders += 1;
	void acquire();
	let done = false;
	return () => {
		if (done) return;
		done = true;
		holders -= 1;
		if (holders === 0) void releaseSentinel();
	};
}
