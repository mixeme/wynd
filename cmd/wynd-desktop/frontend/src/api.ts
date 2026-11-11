// Мост к Go: Wails кладёт методы App в window.go.main.App. Оболочка проверяет
// адреса, хранит независимые источники интерфейса и переключает текущий сервер.

export interface CheckResult {
	origin: string;
	name: string;
	version: string;
	registration_mode: 'open' | 'invite' | 'closed' | string;
}

export interface Update {
	available: boolean;
	version?: string;
	notes?: string[];
	attestation?: string;
}

interface GoApp {
	Version(): Promise<string>;
	CheckSelfUpdate(): Promise<Update>;
	ApplySelfUpdate(): Promise<string>;
	ReleasesPage(): Promise<void>;
	CheckServer(address: string): Promise<CheckResult>;
	CheckCurrent(): Promise<CheckResult>;
	CurrentServer(): Promise<CheckResult | null>;
	ForgetSelected(): Promise<void>;
	PreviewServer(address: string): Promise<CheckResult>;
	AddInterface(address: string): Promise<CheckResult>;
	RemoveInterface(origin: string): Promise<void>;
	ListServers(): Promise<CheckResult[]>;
	SelectServer(origin: string): Promise<CheckResult>;
}

interface Runtime {
	WindowMinimise(): void;
	WindowToggleMaximise(): void;
	WindowIsMaximised(): Promise<boolean>;
	Quit(): void;
}

function bridge(): GoApp | undefined {
	return (window as unknown as { go?: { main?: { App?: GoApp } } }).go?.main?.App;
}

export function runtime(): Runtime | undefined {
	return (window as unknown as { runtime?: Runtime }).runtime;
}

// Версия окна для заголовка: та же, что проверяет самообновление.
export async function currentVersion(): Promise<string> {
	return (await bridge()?.Version()) ?? '';
}

// Новая версия окна: спрашивается при запуске; пусто — обновления нет
// или самообновление выключено в сборке.
export async function checkSelfUpdate(): Promise<Update | null> {
	const app = bridge();
	if (!app?.CheckSelfUpdate) return null;
	return app.CheckSelfUpdate();
}

// «Обновить и перезапустить»: «signature» — подпись не сошлась, иначе текст
// отказа; пусто — новая версия запущена, это окно закрывается.
export async function applySelfUpdate(): Promise<string> {
	return (await bridge()?.ApplySelfUpdate()) ?? '';
}

export async function openingReleases(): Promise<void> {
	await bridge()?.ReleasesPage();
}

export async function checkServer(address: string): Promise<CheckResult> {
	const app = bridge();
	if (!app) throw new Error('no bridge');
	return app.CheckServer(address);
}

export async function currentServer(): Promise<CheckResult | null> {
	const app = bridge();
	if (!app) return null;
	return app.CurrentServer();
}

export async function checkCurrent(): Promise<CheckResult> {
	const app = bridge();
	if (!app) throw new Error('no bridge');
	return app.CheckCurrent();
}

export async function forgetSelected(): Promise<void> {
	await bridge()?.ForgetSelected();
}

export async function previewServer(address: string): Promise<CheckResult> {
	const app = bridge();
	if (!app) throw new Error('no bridge');
	return app.PreviewServer(address);
}

export async function addInterface(address: string): Promise<CheckResult> {
	const app = bridge();
	if (!app) throw new Error('no bridge');
	return app.AddInterface(address);
}

export async function removeInterface(origin: string): Promise<void> {
	const app = bridge();
	if (!app) throw new Error('no bridge');
	await app.RemoveInterface(origin);
}

export async function listServers(): Promise<CheckResult[]> {
	return (await bridge()?.ListServers()) ?? [];
}

export async function selectServer(origin: string): Promise<CheckResult> {
	const app = bridge();
	if (!app) throw new Error('no bridge');
	return app.SelectServer(origin);
}
