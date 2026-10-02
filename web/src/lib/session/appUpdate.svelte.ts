/**
 * Новая версия приложения (план 46, C19).
 *
 * Сама она встаёт, когда приложение ушло в фон, или при следующем запуске:
 * открытый экран не перезагружается посреди дела. В «Настройках» её можно
 * поставить сразу — проверить сервер и перезагрузиться на новой версии.
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
		// Новая версия встаёт сама, когда приложение ушло в фон.
		document.addEventListener('visibilitychange', () => {
			if (appUpdate.pending && document.visibilityState === 'hidden') void applyAppUpdate();
		});
	});
}

/** Спросить сервер; true — новая версия скачана и готова. */
export async function checkAppUpdate(): Promise<boolean> {
	if (!registration) return false;
	if (appUpdate.pending) return true;
	try {
		await registration.update();
	} catch {
		return false;
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
	return appUpdate.pending;
}

/** Перезагрузиться на новой версии. */
export async function applyAppUpdate(): Promise<void> {
	appUpdate.pending = false;
	await update?.(true);
}
