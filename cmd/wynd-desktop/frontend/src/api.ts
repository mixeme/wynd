// Мост к Go: Wails v3 зовёт сервис main.App по имени (те же имена, что в
// сгенерированных биндингах); адаптер написан руками, чтобы проверки фронтенда
// не требовали CLI wails3. mock-стенд по-прежнему подменяет мост через window.go.
// Оболочка проверяет адреса, хранит независимые источники интерфейса и
// переключает текущий сервер.
import { Application, Call, Events, Window as ThisWindow } from '@wailsio/runtime';

export interface CheckResult {
	origin: string;
	name: string;
	version: string;
	registration_mode: 'open' | 'invite' | 'closed' | string;
}

/** Копии по расписанию (план 66, функция 7). */
export interface BackupConfig {
	enabled: boolean;
	folder: string;
	schedule: 'daily' | 'weekly' | 'monthly' | 'custom';
	custom_days?: number;
	keep_mode: 'count' | 'days';
	keep: number;
	last_at?: string;
	last_circles?: number;
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
	RequestClose(): Promise<void>;
	Notify(title: string, body: string): Promise<void>;
	TakePendingInvite(): Promise<string>;
	GetBackupConfig(): Promise<BackupConfig>;
	SaveBackupConfig(cfg: BackupConfig): Promise<BackupConfig>;
	ChooseBackupFolder(): Promise<string>;
	OpenBackupFolder(): Promise<void>;
}

interface Runtime {
	WindowMinimise(): void;
	WindowToggleMaximise(): void;
	WindowIsMaximised(): Promise<boolean>;
	Quit(): void;
}

const native: GoApp = {
	Version: () => Call.ByName('main.App.Version'),
	CheckSelfUpdate: () => Call.ByName('main.App.CheckSelfUpdate'),
	ApplySelfUpdate: () => Call.ByName('main.App.ApplySelfUpdate'),
	ReleasesPage: () => Call.ByName('main.App.ReleasesPage'),
	CheckServer: (address) => Call.ByName('main.App.CheckServer', address),
	CheckCurrent: () => Call.ByName('main.App.CheckCurrent'),
	CurrentServer: () => Call.ByName('main.App.CurrentServer'),
	ForgetSelected: () => Call.ByName('main.App.ForgetSelected'),
	PreviewServer: (address) => Call.ByName('main.App.PreviewServer', address),
	AddInterface: (address) => Call.ByName('main.App.AddInterface', address),
	RemoveInterface: (origin) => Call.ByName('main.App.RemoveInterface', origin),
	ListServers: () => Call.ByName('main.App.ListServers'),
	SelectServer: (origin) => Call.ByName('main.App.SelectServer', origin),
	RequestClose: () => Call.ByName('main.App.RequestClose'),
	Notify: (title, body) => Call.ByName('main.App.Notify', title, body),
	TakePendingInvite: () => Call.ByName('main.App.TakePendingInvite'),
	GetBackupConfig: () => Call.ByName('main.App.GetBackupConfig'),
	SaveBackupConfig: (cfg) => Call.ByName('main.App.SaveBackupConfig', cfg),
	ChooseBackupFolder: () => Call.ByName('main.App.ChooseBackupFolder'),
	OpenBackupFolder: () => Call.ByName('main.App.OpenBackupFolder'),
};

function bridge(): GoApp | undefined {
	return (window as unknown as { go?: { main?: { App?: GoApp } } }).go?.main?.App ?? native;
}

export function runtime(): Runtime {
	// mock-стенд задаёт window.runtime в виде v2; настоящее окно берёт v3.
	const mock = (window as unknown as { runtime?: Runtime }).runtime;
	if (mock) return mock;
	return {
		WindowMinimise: () => void ThisWindow.Minimise(),
		WindowToggleMaximise: () => void ThisWindow.ToggleMaximise(),
		WindowIsMaximised: () => ThisWindow.IsMaximised(),
		Quit: () => void Application.Quit(),
	};
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

// Системный toast по сигналу клиента (план 66): мост сам гасит его, когда
// интерфейс на виду. В браузере оболочки нет — молча.
export async function notifyWindow(title: string, body: string): Promise<void> {
	await bridge()?.Notify(title, body);
}

// Копии по расписанию (план 66, функция 7).
export async function getBackupConfig(): Promise<BackupConfig> {
	return (await bridge()?.GetBackupConfig()) ?? {
		enabled: false,
		folder: '',
		schedule: 'daily',
		keep_mode: 'count',
		keep: 10
	};
}

export async function saveBackupConfig(cfg: BackupConfig): Promise<BackupConfig> {
	return (await bridge()?.SaveBackupConfig(cfg)) ?? cfg;
}

export async function chooseBackupFolder(): Promise<string> {
	return (await bridge()?.ChooseBackupFolder()) ?? '';
}

export async function openBackupFolder(): Promise<void> {
	await bridge()?.OpenBackupFolder();
}

// Приглашение из схемы wynd: (план 66, функция 4). Разбор адреса здесь —
// в тестах без Wails; оболочке уходит уже https-адрес приглашения.
export interface InviteLink {
	origin: string;
	path: string;
}

export function parseInviteLink(arg: string): InviteLink | null {
	if (!/^wynd:/i.test(arg)) return null;
	let rest = arg.slice(arg.indexOf(':') + 1);
	rest = rest.replace(/^\/\//, '');
	if (!/^open\?/i.test(rest)) return null;
	let urlParam = '';
	for (const pair of rest.slice(5).split('&')) {
		const [key, value] = pair.split('=');
		if (key === 'url') {
			urlParam = decodeURIComponent(value ?? '');
			break;
		}
	}
	try {
		const target = new URL(urlParam);
		if (target.protocol !== 'https:') return null;
		return { origin: target.origin, path: target.pathname + target.search };
	} catch {
		return null;
	}
}

/** Живое событие схемы wynd: от Go (второй запуск); null — слушателя нет. */
export function onInviteLink(handler: (link: InviteLink) => void): () => void {
	const off = Events.On('wynd:invite', (event: { data?: unknown }) => {
		const link = typeof event?.data === 'string' ? parseInviteLink(event.data) : null;
		if (link) handler(link);
	});
	return off;
}

/** Адрес, пришедший до загрузки оболочки: событие могло её не застать. */
export async function takePendingInvite(): Promise<InviteLink | null> {
	const arg = await bridge()?.TakePendingInvite();
	return arg ? parseInviteLink(arg) : null;
}

export async function openingReleases(): Promise<void> {
	await bridge()?.ReleasesPage();
}

// «Крестик» окна: со значком в трее окно прячется, без значка приложение
// закрывается — решает Go (RequestClose).
export async function closeWindow(): Promise<void> {
	await bridge()?.RequestClose();
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
