import { androidPort, androidRequest, type AndroidUpdateStatus } from './bridge';

export const androidUpdate = $state<
	AndroidUpdateStatus & { supported: boolean; busy: boolean; hidden: boolean }
>({
	protocol: 0,
	capabilities: { updates: false, developmentUpdateBypass: false },
	supported: false,
	busy: false,
	hidden: false,
	version: '',
	debug: false,
	development: false,
	immediately: false,
	state: 'idle',
	available: false,
	nextVersion: '',
	ready: false,
	error: '',
	blocked: false,
	installing: false,
	permission: false
});

function accept(status: AndroidUpdateStatus): void {
	if (androidUpdate.nextVersion !== status.nextVersion) androidUpdate.hidden = false;
	Object.assign(androidUpdate, status, { development: status.development ?? status.debug });
}

export async function checkAndroidUpdate(): Promise<void> {
	if (!androidUpdate.supported || androidUpdate.busy || androidUpdate.installing) return;
	androidUpdate.busy = true;
	androidUpdate.state = 'checking';
	try {
		accept(await androidRequest('check'));
	} catch {
		androidUpdate.error = 'network';
		androidUpdate.state = 'failed';
	} finally {
		androidUpdate.busy = false;
	}
}

export async function applyAndroidUpdate(): Promise<void> {
	if (androidUpdate.busy || androidUpdate.installing || androidUpdate.blocked) return;
	androidUpdate.busy = true;
	androidUpdate.error = '';
	try {
		if (!androidUpdate.ready) {
			androidUpdate.state = 'downloading';
			accept(await androidRequest('prepare'));
		}
		if (androidUpdate.ready && !androidUpdate.blocked) accept(await androidRequest('install'));
	} catch {
		androidUpdate.error = 'network';
		androidUpdate.state = 'failed';
	} finally {
		androidUpdate.busy = false;
	}
}

export async function setAndroidImmediate(value: boolean): Promise<void> {
	if (!androidUpdate.development || androidUpdate.busy || androidUpdate.installing) return;
	androidUpdate.busy = true;
	try {
		accept(await androidRequest('setImmediately', value));
	} catch {
		androidUpdate.error = 'network';
	} finally {
		androidUpdate.busy = false;
	}
}

export function androidUpdateNote(): string {
	if (androidUpdate.error === 'signature') return 'Подпись не сошлась: обновление не установлено';
	if (['checksum', 'package', 'version'].includes(androidUpdate.error))
		return 'Файл обновления не прошёл проверку';
	if (androidUpdate.error === 'cancelled' || androidUpdate.error === 'install')
		return 'Android не завершил установку обновления';
	if (androidUpdate.error) return 'не удалось обновить — попробуйте позже';
	if (androidUpdate.state === 'checking') return 'проверяем…';
	if (androidUpdate.state === 'downloading') return 'скачиваем и проверяем…';
	if (androidUpdate.installing) return 'устанавливаем…';
	if (androidUpdate.permission) return 'разрешите Wynd устанавливать обновления';
	if (androidUpdate.available) return 'установку подтвердит Android';
	return `Wynd ${androidUpdate.version} · это последняя доступная версия`;
}

export function initAndroidUpdates(): () => void {
	if (!androidPort()) return () => {};
	let stopped = false;
	let lastCheck = 0;
	const refresh = async () => {
		try {
			const status = await androidRequest('status');
			if (stopped) return;
			if (status.protocol !== 1 || !status.capabilities?.updates) return;
			androidUpdate.supported = true;
			accept(status);
			if (!status.ready && !status.installing && Date.now() - lastCheck >= 15 * 60_000) {
				lastCheck = Date.now();
				await checkAndroidUpdate();
			}
		} catch {
			/* Нет сети или страница уходит; версия не объявляется последней. */
		}
	};
	const focus = () => void refresh();
	window.addEventListener('focus', focus);
	const timer = setInterval(() => {
		if (
			!document.hidden &&
			!androidUpdate.busy &&
			(androidUpdate.installing || androidUpdate.permission)
		)
			void refresh();
	}, 2000);
	void refresh();
	return () => {
		stopped = true;
		clearInterval(timer);
		window.removeEventListener('focus', focus);
	};
}
