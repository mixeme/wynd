// Сцены стенда: каждая — ответы подставного Go и путь до экрана щелчками.
// Имя сцены — кадр макета docs/visual/installer.html, где он есть.
import {
	type Fake,
	movePlan,
	never,
	page,
	plan,
	progress,
	report,
	service,
	steps,
	toPult
} from './fake';

export type Act =
	| { click: string; nth?: number }
	| { fill: string; value: string }
	| { tab: 'archives' };

export interface Scene {
	name: string;
	fake?: Fake;
	acts?: Act[];
}

const key: Act = { click: 'Ключ с этого компьютера' };
const go: Act[] = [key, { click: 'Подключиться' }];
const domain: Act[] = [{ fill: 'family.example.ru', value: 'family.example.ru' }, { click: 'Проверить' }];

const blocks = {
	...report,
	report: {
		...report.report,
		findings: [
			{ id: 'os', level: 'ok' as const, text: 'Debian 13' },
			{
				id: 'ports',
				level: 'block' as const,
				text: 'Порты 80 и 443 заняты',
				advice: 'Их держит программа, которую мы не знаем. Освободите порты и проверьте снова.',
				copy: 'ss -ltnp | grep -E ":80 |:443 "',
				after: 'Команда покажет, какая программа держит порты.'
			}
		],
		raw: '## os\nDebian 13\n## ports\nLISTEN 0 511 0.0.0.0:80'
	}
};

const wyndHere = {
	...report,
	report: { ...report.report, findings: [...(report.report.findings ?? []), { id: 'wynd', level: 'ok' as const, text: 'Wynd уже стоит', plan: 'оставим как есть' }] }
};

const pult = page();
// Wynd за чужим Apache, файл сайта ставили не мы (кадры 5е, 5е2).
const foreignApache = {
	...report,
	report: {
		...report.report,
		proxy: 'apache',
		foreign_site: ['/etc/apache2/sites-enabled/wynd.conf'],
		findings: [{ id: 'wynd', level: 'ok' as const, text: 'Wynd' }]
	}
};
const stopped = page({ running: false, stopped_at: '2026-10-24T09:00:00Z' }, { latest: undefined });
const interrupted = page({
	running: false,
	update: { from: '0.43.2', to: '0.43.3', ref: 'v0.43.3', stopped: true, start_after: true, started: '2026-10-24T11:05:00Z', copied: true }
});
const noRoom = page({ free_bytes: 20 * 1024 ** 2 });
const moving = page({ running: false, move: { to: '203.0.113.7', started: '2026-10-26T12:28:00Z', transferred: true } });

const doneSteps = steps(['Docker', 'done'], ['Wynd', 'done'], ['Сертификат', 'done']);

const archiveSummary = { circle_name: 'Семья', from: '2024-01-03', to: '2026-10-20', posts: 412, duplicates: 3, bytes: 1.2 * 1024 ** 3 };
const archiveFile = (name: string, ok = true) => ({
	path: `C:\\Архивы\\${name}`,
	name,
	ok,
	error: ok ? undefined : 'Это не архив Wynd',
	summary: archiveSummary
});

export const SCENES: Scene[] = [
	{ name: '1-connect' },
	{ name: '1-connect-key', acts: [key] },
	{
		name: '1-connect-fail',
		fake: { Connect: () => ({ status: 'error', host: 'x', message: 'Сервер назвал не тот отпечаток', fingerprint: 'SHA256:abc', advice: 'Не подключайтесь, пока не узнаете у хостинга, что случилось.' }) },
		acts: [{ click: 'Подключиться' }]
	},
	{ name: '1a-fingerprint', fake: { Connect: () => ({ status: 'unknown_host', host: 'x', fingerprint: 'SHA256:Qm1x0tFv8kQb2z3N7cWm0r5vP9yLwH4dJ6sE1aK2gU' }) }, acts: [{ click: 'Подключиться' }] },
	{ name: '2-inspect-wait', fake: { Inspect: never }, acts: go },
	{ name: '2-inspect-failed', fake: { Inspect: () => ({ ok: false, message: 'Осмотр не прошёл', advice: 'Связь оборвалась.', report: { findings: null, addresses: null, raw: '' } }) }, acts: go },
	{ name: '2-inspect', acts: go },
	{ name: '2-inspect-domain-ok', acts: [...go, ...domain] },
	{
		name: '2-inspect-domain-bad',
		fake: { CheckDomain: (d: string) => ({ domain: d, level: 'block', text: `${d} пока никуда не указывает`, advice: 'У регистратора домена добавьте запись типа A.' }) },
		acts: [...go, ...domain]
	},
	{ name: '2a-blocks', fake: { Inspect: () => blocks }, acts: go },
	{ name: '2e-raw', fake: { Inspect: () => blocks }, acts: [...go, { click: 'подробнее, что проверили' }] },
	{ name: '2b-remove', fake: { Inspect: () => wyndHere, ServerPage: () => page({ container: false }) }, acts: [...go, { click: 'Убрать Wynd с сервера' }] },
	{ name: '3-plan', acts: [...go, ...domain, { click: 'Дальше — план' }] },
	{ name: '3-plan-blocked', fake: { MakePlan: () => ({ ...plan, ok: false, message: 'Эту версию ещё нельзя поставить', advice: 'Подождите выпуска.' }) }, acts: [...go, ...domain, { click: 'Дальше — план' }] },
	{ name: '4-install', acts: [...go, ...domain, { click: 'Дальше — план' }, { click: 'Установить' }] },
	{
		name: '4a-install-failed',
		fake: { StartInstall: () => progress(steps(['Docker', 'done'], ['Wynd', 'failed']), { running: false, failure: { step: 's1', message: 'Wynd не скачался', advice: 'Хранилище образов не ответило.' } }) },
		acts: [...go, ...domain, { click: 'Дальше — план' }, { click: 'Установить' }]
	},
	{
		name: '4a-install-rollback',
		fake: { Rollback: () => ({ ok: false, message: 'Откат не прошёл', advice: 'Docker не ответил', log: [{ command: 'docker compose down', output: 'Cannot connect to the Docker daemon' }] }), StartInstall: () => progress(steps(['Docker', 'done'], ['Wynd', 'failed']), { running: false, failure: { step: 's1', message: 'Wynd не скачался', advice: 'Хранилище образов не ответило.' } }) },
		acts: [...go, ...domain, { click: 'Дальше — план' }, { click: 'Установить' }, { click: 'Откатить установку' }, { click: 'Удалить данные Wynd — записи, фотографии, учётные записи — и сертификат сайта' }, { click: 'Да, откатить' }, { click: 'подробности' }]
	},
	{ name: '5-done-link', fake: { StartInstall: () => progress(doneSteps, { running: false, done: true, link: 'https://family.example.ru/setup#k3J9xQ' }) }, acts: [...go, ...domain, { click: 'Дальше — план' }, { click: 'Установить' }] },
	{ name: '5-done-nolink', fake: { StartInstall: () => progress(doneSteps, { running: false, done: true, site: 'https://family.example.ru' }) }, acts: [...go, ...domain, { click: 'Дальше — план' }, { click: 'Установить' }] },
	{ name: '7a-server', fake: toPult(pult), acts: go },
	{ name: '7a2-server-stopped', fake: toPult(stopped), acts: go },
	{ name: '7a-ask-stop', fake: toPult(pult), acts: [...go, { click: 'Остановить' }] },
	{ name: '7e-interrupted', fake: toPult(interrupted), acts: go },
	{ name: '7a-service-error', fake: { ...toPult(pult), Service: () => service(pult, false) }, acts: [...go, { click: 'Перезапустить' }, { click: 'подробности' }] },
	{ name: '7a-remove', fake: { ...toPult(pult), Inspect: () => ({ ...report, report: { ...report.report, proxy: 'nginx', findings: [{ id: 'wynd', level: 'ok', text: 'Wynd' }] } }) }, acts: [...go, { click: 'Убрать Wynd' }] },
	{
		name: '5e-remove-foreign-site',
		fake: { ...toPult(pult), Inspect: () => foreignApache },
		acts: [...go, { click: 'Убрать Wynd' }]
	},
	{
		name: '5e2-removed-foreign-site',
		fake: { ...toPult(pult), Inspect: () => foreignApache, Rollback: () => ({ ok: true, log: null, report: { ...foreignApache.report, findings: report.report.findings } }) },
		acts: [...go, { click: 'Убрать Wynd' }, { click: 'Да, убрать' }]
	},
	{ name: '7a-selfbar', fake: { ...toPult(pult), CheckSelfUpdate: () => ({ available: true, version: '0.2.0', notes: ['Обновляется сама'], attestation: 'https://github.com/' }) }, acts: go },
	{ name: '7a-selfbar-hidden', fake: { ...toPult(pult), CheckSelfUpdate: () => ({ available: true, version: '0.2.0', notes: ['Обновляется сама'] }) }, acts: [...go, { click: '✕' }] },
	{ name: '10b-self', fake: { CheckSelfUpdate: () => ({ available: true, version: '0.2.0', notes: ['Обновляется сама', 'Переезд узнаёт обрыв'], attestation: 'https://github.com/' }) }, acts: [{ click: 'Обновить' }] },
	{ name: '10c-self-bad', fake: { CheckSelfUpdate: () => ({ available: true, version: '0.2.0' }), ApplySelfUpdate: () => 'signature' }, acts: [{ click: 'Обновить' }, { click: 'Обновить и перезапустить' }] },
	{ name: '7b-uplan', fake: toPult(pult), acts: [...go, { click: 'Обновить' }] },
	{ name: '7b-uplan-stopped', fake: toPult(page({ running: false, stopped_at: '2026-10-24T09:00:00Z' })), acts: [...go, { click: 'Обновить' }] },
	{ name: '7b-uplan-build', fake: toPult(pult), acts: [...go, { click: 'Обновить' }, { click: 'Собрать самому' }] },
	{ name: '7b2-noroom', fake: toPult(noRoom), acts: [...go, { click: 'Обновить' }] },
	{ name: '7c-update', fake: { ...toPult(pult), StartUpdate: () => progress(steps(['Скачаем', 'done'], ['Копия', 'running', '12 МБ'], ['Запуск', 'pending'])) }, acts: [...go, { click: 'Обновить' }, { click: 'Обновить' }] },
	{
		name: '7d-update-failed',
		fake: { ...toPult(pult), StartUpdate: () => progress(steps(['Запуск', 'failed']), { running: false, failure: { step: 'start', message: 'Новая версия не запустилась', advice: 'Wynd вышел с ошибкой.' } }) },
		acts: [...go, { click: 'Обновить' }, { click: 'Обновить' }, { click: 'подробности' }]
	},
	{ name: '7g-udone', fake: { ...toPult(pult), StartUpdate: () => progress(doneSteps, { running: false, done: true }) }, acts: [...go, { click: 'Обновить' }, { click: 'Обновить' }] },
	{ name: '8a-bplan', fake: toPult(pult), acts: [...go, { click: 'сделать' }] },
	{
		name: '8a2-bplan-encrypt',
		fake: { ...toPult(pult), PlanBackup: () => ({ ok: true, total: 2.4 * 1024 ** 3, fetch: 300 * 1024 ** 2, prev: '2026-10-20T10:00:00Z' }) },
		acts: [...go, { click: 'сделать' }, { click: 'Зашифровать паролем' }]
	},
	{ name: '8a-bplan-bad', fake: { ...toPult(pult), PlanBackup: () => ({ ok: false, message: 'В эту папку нельзя писать', total: 0, fetch: 0 }) }, acts: [...go, { click: 'сделать' }] },
	{ name: '8b-backup', fake: toPult(pult), acts: [...go, { click: 'сделать' }, { click: 'Сделать бэкап' }] },
	{
		name: '8g-backup-failed',
		fake: { ...toPult(pult), PlanBackup: () => ({ ok: true, total: 1, fetch: 1, prev: '2026-10-20T10:00:00Z' }), StartBackup: () => progress(steps(['Скачиваем', 'failed']), { running: false, failure: { step: 'fetch', message: 'Связь с сервером прервалась', advice: 'Скачано 300 из 2400 МБ.' } }) },
		acts: [...go, { click: 'сделать' }, { click: 'Сделать бэкап' }]
	},
	{ name: '8v-bdone', fake: { ...toPult(pult), StartBackup: () => progress(doneSteps, { running: false, done: true }) }, acts: [...go, { click: 'сделать' }, { click: 'Сделать бэкап' }] },
	{ name: '9a-mconnect', fake: toPult(pult), acts: [...go, { click: 'начать' }] },
	{ name: '9a-mconnect-fail', fake: { ...toPult(pult), MoveConnect: () => ({ status: 'error', host: 'x', message: 'Не подключились', advice: 'Проверьте адрес.' }) }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }] },
	{ name: '9a-mfinger', fake: { ...toPult(pult), MoveConnect: () => ({ status: 'unknown_host', host: 'x', fingerprint: 'SHA256:Zx9' }) }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }] },
	{ name: '9b-mplan-wait', fake: { ...toPult(pult), MovePlan: never }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }] },
	{ name: '9b-mplan-blocked', fake: { ...toPult(pult), MovePlan: () => ({ ...movePlan, ok: false, message: 'На новом сервере уже есть веб-сервер', advice: 'Уберите nginx.' }) }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }] },
	{ name: '9b-mplan', fake: toPult(pult), acts: [...go, { click: 'начать' }, { click: 'Подключиться' }] },
	{ name: '9b2-mplan-data', fake: { ...toPult(pult), MovePlan: () => ({ ...movePlan, data_only: true, old_proxy: 'nginx' }) }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }] },
	{ name: '9v-move', fake: { ...toPult(pult), StartMove: () => progress(steps(['Wynd на новом', 'done'], ['Сайт на старом остановлен', 'running'], ['Перенос', 'pending'])) }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }, { click: 'Переехать' }] },
	{
		name: '9v-move-dns',
		fake: { ...toPult(pult), StartMove: () => ({ ...progress(steps(['Перенос', 'done'])), steps: [{ id: 'dns', title: 'Ждём новый адрес', status: 'running', note: 'family.example.ru пока указывает на 45.144.64.241' }] }) },
		acts: [...go, { click: 'начать' }, { click: 'Подключиться' }, { click: 'Переехать' }]
	},
	{
		name: '9v3-move-dns-unknown',
		fake: { ...toPult(pult), StartMove: () => ({ ...progress(steps(['Перенос', 'done'])), steps: [{ id: 'dns', title: 'Ждём новый адрес', status: 'running', note: 'пока не удалось узнать, куда указывает family.example.ru' }] }) },
		acts: [...go, { click: 'начать' }, { click: 'Подключиться' }, { click: 'Переехать' }]
	},
	{ name: '9v-move-failed', fake: { ...toPult(pult), StartMove: () => progress(steps(['Перенос', 'failed']), { running: false, failure: { step: 'transfer', message: 'Перенос прервался', advice: 'Связь между серверами оборвалась.' } }) }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }, { click: 'Переехать' }, { click: 'подробности' }] },
	{ name: '9v2-unfinished', fake: toPult(moving), acts: go },
	{ name: '9v2-continue-connect', fake: toPult(moving), acts: [...go, { click: 'Продолжить переезд' }] },
	{
		name: '9v2-continue',
		fake: {
			...toPult(moving),
			MovePlan: () => ({ ...movePlan, resume: true }),
			StartMove: () => progress(steps(['Wynd на новом', 'done'], ['Сайт на старом остановлен', 'done'], ['Папка Wynd перенесена', 'done', 'перенесено раньше'], ['Ждём новый адрес', 'running']))
		},
		acts: [...go, { click: 'Продолжить переезд' }, { click: 'Подключиться' }]
	},
	{
		name: '9v2-cancel-refused',
		fake: { ...toPult(moving), CancelMove: () => ({ ...service(moving, false), message: 'Отменить переезд уже нельзя', advice: 'Wynd на новом сервере запущен, там могли писать. Продолжите переезд.' }) },
		acts: [...go, { click: 'Отменить переезд' }, { click: 'Подключиться' }]
	},
	{ name: '9v2-cancelled', fake: { ...toPult(moving), CancelMove: () => service(pult) }, acts: [...go, { click: 'Отменить переезд' }, { click: 'Подключиться' }] },
	{ name: '9g-mdone', fake: { ...toPult(pult), StartMove: () => progress(doneSteps, { running: false, done: true }) }, acts: [...go, { click: 'начать' }, { click: 'Подключиться' }, { click: 'Переехать' }] },
	{ name: '11a-archives', acts: [{ tab: 'archives' }] },
	{
		name: '11b-merge',
		fake: { PickArchives: () => ['C:\\Архивы\\a.zip', 'C:\\Архивы\\b.zip'], MergeInfo: () => ({ files: [archiveFile('Семья 2024.zip'), archiveFile('фото.zip', false)], ok: true, summary: archiveSummary, suggest: 'Семья 2024–2026.zip' }) },
		acts: [{ tab: 'archives' }, { click: 'выбрать', nth: 0 }, { click: '+ добавить архив' }]
	},
	{
		name: '11v-split',
		fake: { PickArchive: () => 'C:\\Архивы\\Семья.zip', SplitInfo: () => ({ file: archiveFile('Семья.zip'), years: [{ year: 2025, summary: archiveSummary }, { year: 2026, summary: archiveSummary }] }) },
		acts: [{ tab: 'archives' }, { click: 'выбрать', nth: 1 }, { click: 'Выбрать архив' }]
	},
	{
		name: '11v-split-range',
		fake: { PickArchive: () => 'C:\\Архивы\\Семья.zip', SplitInfo: () => ({ file: archiveFile('Семья.zip'), years: [] }) },
		acts: [{ tab: 'archives' }, { click: 'выбрать', nth: 1 }, { click: 'Выбрать архив' }, { click: 'За срок' }]
	},
	{
		name: '11g-full',
		fake: { PickArchive: () => 'C:\\Архивы\\Семья.wynd', FullInfo: () => ({ ok: true, name: 'Семья.wynd', bytes: 3 * 1024 ** 3, encrypted: true }) },
		acts: [{ tab: 'archives' }, { click: 'выбрать', nth: 2 }, { click: 'Выбрать файл .wynd' }]
	},
	{
		name: '11-done',
		fake: { PickArchive: () => 'C:\\Архивы\\Семья.wynd', FullInfo: () => ({ ok: true, name: 'Семья.wynd', bytes: 1, encrypted: false }), Readable: () => ({ ok: true, files: ['C:\\Архивы\\Семья — весь круг.zip'] }) },
		acts: [{ tab: 'archives' }, { click: 'выбрать', nth: 2 }, { click: 'Выбрать файл .wynd' }, { click: 'Сделать ленту' }]
	},
	{ name: '11-error', fake: { PickArchive: () => 'C:\\a.wynd', FullInfo: () => ({ ok: true, name: 'a.wynd', bytes: 1 }), Readable: () => ({ ok: false, message: 'Файл повреждён' }) }, acts: [{ tab: 'archives' }, { click: 'выбрать', nth: 2 }, { click: 'Выбрать файл .wynd' }, { click: 'Сделать ленту' }] }
];

