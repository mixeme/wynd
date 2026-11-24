// Автозапуск живёт в оболочке (HKCU\...\Run), а настройка — в обычном
// «Настройки → Приложение» внутри iframe. Окно и iframe — один origin,
// поэтому iframe зовёт методы оболочки через window.parent.go.
import { desktopBridge } from './bridge';

export interface AutostartStatus {
	enabled: boolean;
	stale: boolean;
	command: string;
}

interface DesktopApp {
	Autostart(): Promise<AutostartStatus>;
	SetAutostart(enabled: boolean): Promise<AutostartStatus>;
}

function desktopApp(): DesktopApp | undefined {
	if (typeof window === 'undefined' || !desktopBridge()) return undefined;
	const top = window.parent as unknown as { go?: { main?: { App?: DesktopApp } } };
	return top.go?.main?.App;
}

export interface AutostartBridge {
	get(): Promise<AutostartStatus>;
	set(enabled: boolean): Promise<AutostartStatus>;
}

export function autostartBridge(): AutostartBridge | undefined {
	const app = desktopApp();
	if (!app) return undefined;
	return {
		get: () => app.Autostart(),
		set: (enabled) => app.SetAutostart(enabled)
	};
}
