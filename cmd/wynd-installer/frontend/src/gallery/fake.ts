// Подставной Go для стенда (gallery.html): каждая сцена задаёт ответы сама,
// остальное — безобидные значения по умолчанию.
import type {
	ConnectResult,
	InspectResult,
	MovePlan,
	Plan,
	Progress,
	ServerPage,
	ServiceResult,
	StepState
} from '../api';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Fake = Record<string, (...args: any[]) => unknown>;

/** Ответ, который не приходит никогда: экран «ждём». */
export const never = () => new Promise(() => {});

export const connected: ConnectResult = { status: 'connected', host: '45.144.64.241' };

export const steps = (...s: [string, StepState['status'], string?][]): StepState[] =>
	s.map(([title, status, note], i) => ({ id: `s${i}`, title, status, note }));

export const report: InspectResult = {
	ok: true,
	report: {
		findings: [
			{ id: 'os', level: 'ok', text: 'Debian 13', plan: 'подходит' },
			{ id: 'docker', level: 'note', text: 'Docker нет', plan: 'поставим' },
			{ id: 'disk', level: 'ok', text: 'Свободно 18 ГБ' }
		],
		addresses: ['45.144.64.241'],
		raw: '## os\nDebian GNU/Linux 13 (trixie)\n## docker\nnot found\n## disk\n/dev/vda1 20G 2G 18G'
	}
};

export const plan: Plan = {
	ok: true,
	domain: 'family.example.ru',
	version: '0.43.3',
	install: ['Docker из хранилища Debian', 'Wynd 0.43.3 и Caddy — контейнерами'],
	change: ['Часовой пояс — Europe/Moscow'],
	keep: ['SSH и пользователи сервера'],
	duration: 'обычно 2–3 минуты',
	build: false,
	canBuild: true,
	unpublished: false
};

export const page = (over: Partial<ServerPage['state']> = {}, top: Partial<ServerPage> = {}): ServerPage => ({
	ok: true,
	state: {
		installed: true,
		version: '0.43.2',
		domain: 'family.example.ru',
		running: true,
		container: true,
		free_bytes: 4.3 * 1024 ** 3,
		...over
	},
	host: '198.51.100.4',
	latest: '0.43.3',
	copy_bytes: 12 * 1024 ** 2,
	live_copy: true,
	can_build: true,
	...top
});

export const service = (p: ServerPage, ok = true): ServiceResult => ({
	ok,
	message: ok ? undefined : 'Wynd не запустился',
	advice: ok ? undefined : 'Контейнер вышел с ошибкой — подробности ниже',
	log: [{ command: 'docker compose up -d', output: 'Error: port is already allocated' }],
	page: p
});

export const progress = (s: StepState[], over: Partial<Progress> = {}): Progress => ({
	steps: s,
	log: [{ command: 'apt-get install -y docker.io', output: 'Reading package lists... Done' }],
	running: true,
	done: false,
	...over
});

export const movePlan: MovePlan = {
	ok: true,
	domain: 'family.example.ru',
	version: '0.43.2',
	new_ip: '203.0.113.7',
	old_ip: '45.144.64.241',
	size: 1024 ** 3 * 2.4,
	data_only: false
};

/** Пульт: подключение по ключу сразу ведёт на страницу сервера. */
export const toPult = (p: ServerPage): Fake => ({
	Connect: () => connected,
	Inspect: () => ({ ...report, report: { ...report.report, findings: [{ id: 'wynd', level: 'ok', text: 'Wynd 0.43.2' }] } }),
	ServerPage: () => p
});

export function makeBackend(over: Fake): Fake {
	const base: Fake = {
		HasSavedPassword: () => false,
		Connect: () => connected,
		ConfirmHost: () => connected,
		Inspect: () => report,
		CheckDomain: (d: string) => ({ domain: d, level: 'ok', text: 'указывает на этот сервер' }),
		MakePlan: () => plan,
		ChooseBuild: (on: boolean) => ({ ...plan, build: on }),
		StartInstall: () => progress(steps(['Docker', 'running'], ['Wynd', 'pending'])),
		InstallProgress: () => progress(steps(['Docker', 'running'], ['Wynd', 'pending'])),
		Rollback: () => ({ ok: true, log: null }),
		OpenLink: () => undefined,
		CopyText: () => true,
		Disconnect: () => undefined,
		Version: () => '0.1.0',
		ServerPage: () => page(),
		Service: () => service(page()),
		StartUpdate: () => progress(steps(['Скачаем', 'running'])),
		ReturnPrevious: () => service(page()),
		PickArchives: () => [],
		PickArchive: () => '',
		PickSave: () => '',
		PickFolder: () => '',
		MergeInfo: () => ({ files: [], ok: false, summary: { circle_name: '', posts: 0, duplicates: 0, bytes: 0 } }),
		Merge: () => ({ ok: true, files: [] }),
		SplitInfo: () => undefined,
		Split: () => ({ ok: true, files: [] }),
		FullInfo: () => ({ ok: false }),
		Readable: () => ({ ok: true, files: [] }),
		ShowInFolder: () => undefined,
		PlanBackup: () => ({ ok: true, total: 2.4 * 1024 ** 3, fetch: 2.4 * 1024 ** 3 }),
		StartBackup: () => progress(steps(['Скачиваем', 'running'])),
		DefaultBackupFolder: () => 'C:\\Users\\Мария\\Documents\\Wynd\\family.example.ru',
		MoveConnect: () => connected,
		MoveConfirmHost: () => connected,
		MovePlan: () => movePlan,
		StartMove: () => progress(steps(['Wynd на новом сервере', 'running'])),
		MoveCheckNow: () => undefined,
		CancelMove: () => service(page()),
		MoveAdopt: () => page(),
		CheckSelfUpdate: () => ({ available: false }),
		ApplySelfUpdate: () => '',
		ReleasesPage: () => undefined
	};
	const all = { ...base, ...over };
	// Окно спрашивает, как идёт работа, каждые полсекунды: пусть слышит то же,
	// что ответил запуск, — снимок не зависит от того, успел ли опрос.
	let last: unknown;
	for (const k of ['StartInstall', 'StartUpdate', 'StartBackup', 'StartMove']) {
		const start = all[k];
		all[k] = (...a: unknown[]) => (last = start(...a));
	}
	if (!over.InstallProgress) all.InstallProgress = () => last;
	// Go отвечает обещанием — так и здесь, даже на мгновенные ответы.
	return Object.fromEntries(Object.entries(all).map(([k, f]) => [k, (...a: unknown[]) => Promise.resolve(f(...a))]));
}
