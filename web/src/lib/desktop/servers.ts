// Добавление чужого сервера в окне: сначала разрешаем адрес для /__srv,
// потом проверяем и входим; после входа сохраняем имя. В браузере моста нет.
import { desktopBridge } from './bridge';

interface DesktopApp {
	AllowServer(address: string): Promise<string>;
	AddServer(origin: string, name: string, version: string, mode: string): Promise<void>;
}

function desktopApp(): DesktopApp | undefined {
	if (typeof window === 'undefined' || !desktopBridge()) return undefined;
	const top = window.parent as unknown as { go?: { main?: { App?: DesktopApp } } };
	return top.go?.main?.App;
}

/** Разрешает адрес до проверки: API нового сервера пойдёт через /__srv. */
export async function allowDesktopServer(address: string): Promise<void> {
	try {
		await desktopApp()?.AllowServer(address);
	} catch {
		// Проверка адреса покажет «Сервер не отвечает»; молчание здесь — не отказ.
	}
}

/** Открывает в оболочке выбор временного источника интерфейса. */
export function openDesktopInterface(): void {
	if (typeof window === 'undefined' || !desktopBridge()) return;
	window.parent.postMessage({ type: 'wynd:open-interface' }, window.location.origin);
}

/** Сохраняет имя и версию после входа на другой сервер. */
export async function registerDesktopServer(
	origin: string,
	name: string,
	version = '',
	mode = ''
): Promise<void> {
	if (!origin) return;
	try {
		await desktopApp()?.AddServer(origin, name, version, mode);
	} catch {
		// Список серверов оболочки останется с адресом; вход уже состоялся.
	}
}
