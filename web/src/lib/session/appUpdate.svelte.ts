/**
 * Новая версия приложения (план 46, C19).
 *
 * Сама она встаёт при следующем запуске: скачанный воркер ждёт, пока
 * приложение закроют, и браузер включает его без перезагрузки страницы.
 * Сами страницу не перезагружаем ни на запуске, ни в фоне: веб-приложение
 * Firefox на Android от перезагрузки теряет окно, и следующее нажатие на
 * значок только мигало экраном. В «Настройках» версию можно поставить
 * сразу — проверить сервер и перезагрузиться на новой версии.
 * Неотправленное лежит в очереди IndexedDB и переживает перезагрузку;
 * идущая загрузка продолжится с того же места (сессии загрузки).
 */

type UpdateFn = (reload?: boolean) => Promise<void>;

export const appUpdate = $state({
	/** Новая версия скачана и ждёт. */
	pending: false,
	/** Сервис-воркер есть: проверять можно. */
	supported: false
});

let update: UpdateFn | null = null;
let registration: ServiceWorkerRegistration | undefined;

/** Корневой макет: регистрация сервис-воркера (не на 127.0.0.1). */
export function initAppUpdate(): void {
	void import('virtual:pwa-register').then(({ registerSW }) => {
		update = registerSW({
			immediate: true,
			onNeedRefresh() {
				appUpdate.pending = true;
			},
			onRegisteredSW(_url, reg) {
				registration = reg;
				appUpdate.supported = Boolean(reg);
			}
		});
	});
}

/** Спросить сервер: новая версия готова, это последняя или спросить не вышло. */
export async function checkAppUpdate(): Promise<'available' | 'latest' | 'failed'> {
	if (!registration) return 'failed';
	if (appUpdate.pending) return 'available';
	try {
		await registration.update();
	} catch {
		// Нет сети или сервер не отдал воркер — «последняя версия» была бы неправдой.
		return 'failed';
	}
	// Новый воркер ставится не мгновенно: ждём, пока он встанет в очередь.
	const installing = registration.installing;
	if (installing) {
		await new Promise<void>((resolve) => {
			const done = () => {
				if (installing.state === 'installed' || installing.state === 'redundant') resolve();
			};
			installing.addEventListener('statechange', done);
			setTimeout(resolve, 15_000);
			done();
		});
	}
	if (registration.waiting) appUpdate.pending = true;
	return appUpdate.pending ? 'available' : 'latest';
}

/** Перезагрузиться на новой версии. */
export async function applyAppUpdate(): Promise<void> {
	appUpdate.pending = false;
	await update?.(true);
}
