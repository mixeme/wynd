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

function useRegistration(reg: ServiceWorkerRegistration | undefined): void {
	if (!reg) return;
	registration = reg;
	appUpdate.supported = true;
	if (reg.waiting) appUpdate.pending = true;
}

/** Уже установленный воркер: строка в «Настройках» не ждёт регистрации. */
function useExistingRegistration(): void {
	void navigator.serviceWorker
		?.getRegistration()
		.then(useRegistration)
		.catch(() => {});
}

/** Корневой макет: регистрация сервис-воркера (не на 127.0.0.1). */
export function initAppUpdate(): void {
	useExistingRegistration();
	void import('virtual:pwa-register').then(({ registerSW }) => {
		update = registerSW({
			immediate: true,
			onNeedRefresh() {
				appUpdate.pending = true;
			},
			onRegisteredSW(_url, reg) {
				useRegistration(reg);
			},
			// Сервер не ответил на запуске (нет сети, медленная связь):
			// регистрация падает, но установленный воркер на месте — без
			// этого строка обновления пропадала до следующего запуска.
			onRegisterError() {
				useExistingRegistration();
			}
		});
	});
}

/** Сколько ждать скачивания новой версии, прежде чем отпустить строку. */
const DOWNLOAD_WAIT_MS = 120_000;

/**
 * Спросить сервер: новая версия готова, это последняя или спросить не вышло.
 * Пока новая версия скачивается — onDownloading (строка пишет об этом);
 * не скачалась за DOWNLOAD_WAIT_MS — 'downloading': встанет сама, строку
 * переключит onNeedRefresh.
 */
export async function checkAppUpdate(
	onDownloading?: () => void
): Promise<'available' | 'latest' | 'failed' | 'downloading'> {
	if (!registration) return 'failed';
	if (appUpdate.pending) return 'available';
	if (!registration.installing) {
		try {
			await registration.update();
		} catch {
			// Нет сети или сервер не отдал воркер — «последняя версия» была бы неправдой.
			return 'failed';
		}
	}
	const installing = registration.installing;
	if (installing) {
		onDownloading?.();
		const settled = await new Promise<boolean>((resolve) => {
			const done = () => {
				if (installing.state === 'installed' || installing.state === 'redundant') resolve(true);
			};
			installing.addEventListener('statechange', done);
			setTimeout(() => resolve(false), DOWNLOAD_WAIT_MS);
			done();
		});
		if (!settled && !registration.waiting) return 'downloading';
	}
	if (registration.waiting) appUpdate.pending = true;
	return appUpdate.pending ? 'available' : 'latest';
}

/** Перезагрузиться на новой версии. */
export async function applyAppUpdate(): Promise<void> {
	appUpdate.pending = false;
	await update?.(true);
}
