<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Checkbox from '$ui/workshop/Checkbox.svelte';
	import CommandLog from '$ui/workshop/CommandLog.svelte';
	import CopyLine from '$ui/workshop/CopyLine.svelte';
	import FieldLine from '$ui/workshop/FieldLine.svelte';
	import FootBar from '$ui/workshop/FootBar.svelte';
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';
	import Notice from '$ui/workshop/Notice.svelte';
	import Radio from '$ui/workshop/Radio.svelte';
	import RawSheet from '$ui/workshop/RawSheet.svelte';
	import SelfBar from '$ui/workshop/SelfBar.svelte';
	import StepList from '$ui/workshop/StepList.svelte';
	import StepNav from '$ui/workshop/StepNav.svelte';
	import WorkshopTabs from '$ui/workshop/WorkshopTabs.svelte';
	import {
		backend,
		type ConnectResult,
		type DomainCheck,
		type Finding,
		type InspectResult,
		type LogEntry,
		type LocalBackupPlan,
		type MovePlan,
		type Plan,
		type Progress,
		type SelfUpdate,
		type ServerPage
	} from './api';
	import { rawSections } from './raw';
	import Archives from './Archives.svelte';

	// Вкладки «Серверы» и «Архивы» (владелец, 2026-10-24: «Лучше отдельной вкладкой»).
	let tab = $state<'servers' | 'archives'>('servers');

	// Новая Мастерская (10а–10в): спрашиваем при запуске; «✕» прячет только
	// полосу — тихая строка внизу остаётся, пока не обновились (10а2).
	let selfUp = $state<SelfUpdate | undefined>();
	let selfHidden = $state(false);
	let selfOpen = $state(false);
	let selfBusy = $state(false);
	let selfBad = $state(false);
	let selfError = $state('');
	let ownVersion = $state('');
	void backend()
		.CheckSelfUpdate()
		.then((u) => (selfUp = u.available ? u : undefined))
		.catch(() => undefined);
	void backend()
		.Version()
		.then((v) => (ownVersion = v))
		.catch(() => undefined);
	async function applySelf() {
		if (selfBusy) return;
		selfBusy = true;
		selfError = '';
		try {
			const res = await backend().ApplySelfUpdate();
			if (res === 'signature') selfBad = true;
			else if (res) selfError = res;
		} finally {
			selfBusy = false;
		}
	}

	// Окна установщика (docs/visual/installer.html): 1 — подключение, 1а —
	// отпечаток, 2 — осмотр, 2а — помеха, 3 — план, 4 — установка, 4а — шаг не
	// прошёл, 5 — готово. Пульт сервера (6) — следующий срез.
	type Screen =
		| 'connect'
		| 'fingerprint'
		| 'inspect'
		| 'plan'
		| 'install'
		| 'done'
		| 'server'
		| 'uplan'
		| 'update'
		| 'udone'
		| 'bplan'
		| 'backup'
		| 'bdone'
		| 'mconnect'
		| 'mfinger'
		| 'mplan'
		| 'move'
		| 'mdone';
	const INSTALL_STEPS = ['Сервер', 'Осмотр', 'План', 'Установка', 'Готово'];
	// Обновление со страницы сервера (кадры 7б–7д).
	const UPDATE_STEPS = ['Сервер', 'План', 'Обновление', 'Готово'];
	// Бэкап на этот компьютер (кадры 8а–8г).
	const BACKUP_STEPS = ['Сервер', 'Бэкап', 'Готово'];
	// Переезд на другой сервер (кадры 9а–9г).
	const MOVE_STEPS = ['Новый сервер', 'План', 'Переезд', 'Готово'];

	let screen = $state<Screen>('connect');
	let host = $state('');
	let user = $state('root');
	let password = $state('');
	let useKeys = $state(false);
	let remember = $state(false);
	let makeKey = $state(false);
	let keyNote = $state('');
	let saved = $state(false);
	let busy = $state(false);
	let failure = $state<ConnectResult | undefined>();
	let fingerprint = $state('');

	let inspection = $state<InspectResult | undefined>();
	let showRaw = $state(false);
	let copiedRaw = $state(false);
	let domain = $state('');
	let domainCheck = $state<DomainCheck | undefined>();
	let checkingDomain = $state(false);

	const findings = $derived<Finding[]>(inspection?.report.findings ?? []);
	const blocks = $derived(findings.filter((f) => f.level === 'block'));
	const rawParts = $derived(
		inspection
			? rawSections(
					inspection.report.raw,
					blocks.map((b) => b.id)
				)
			: []
	);
	let plan = $state<Plan | undefined>();
	let planning = $state(false);
	let progress = $state<Progress | undefined>();
	// Страница сервера, где Wynd уже стоит (пульт).
	let page = $state<ServerPage | undefined>();
	// План переезда (9б).
	let mplan = $state<MovePlan | undefined>();
	let showLog = $state(false);
	let askRollback = $state(false);
	let dropData = $state(false);
	let rolling = $state(false);
	let rollbackNote = $state('');
	let rollbackError = $state('');
	let rollbackLog = $state<LogEntry[]>([]);
	let copied = $state(false);
	let copiedLine = $state('');

	const SCREEN_STEP: Record<Screen, number> = {
		connect: 0,
		fingerprint: 0,
		inspect: 1,
		plan: 2,
		install: 3,
		done: 4,
		server: 0,
		uplan: 1,
		update: 2,
		udone: 3,
		bplan: 1,
		backup: 1,
		bdone: 2,
		mconnect: 0,
		mfinger: 0,
		mplan: 1,
		move: 2,
		mdone: 3
	};
	const updating = $derived(screen === 'uplan' || screen === 'update' || screen === 'udone');
	const backingUp = $derived(screen === 'bplan' || screen === 'backup' || screen === 'bdone');
	const moving = $derived(['mconnect', 'mfinger', 'mplan', 'move', 'mdone'].includes(screen));
	const STEPS = $derived(
		updating ? UPDATE_STEPS : backingUp ? BACKUP_STEPS : moving ? MOVE_STEPS : INSTALL_STEPS
	);
	const stepIndex = $derived(SCREEN_STEP[screen]);
	const stepBad = $derived(
		(screen === 'inspect' && (blocks.length > 0 || inspection?.ok === false)) ||
			(screen === 'plan' && plan?.ok === false) ||
			((screen === 'install' || screen === 'update' || screen === 'backup' || screen === 'move') &&
				Boolean(progress?.failure)) ||
			(screen === 'mplan' && mplan?.ok === false) ||
			(screen === 'uplan' && noRoom(page))
	);
	const domainOk = $derived(domainCheck?.level === 'ok');

	// Есть сохранённый пароль — поле можно оставить пустым.
	let savedFor = '';
	async function refreshSaved() {
		const key = `${user.trim()}@${host.trim()}`;
		if (key === savedFor) return;
		savedFor = key;
		saved = host.trim() && user.trim() ? await backend().HasSavedPassword(host, user) : false;
		if (saved) remember = true;
	}

	function applyConnect(res: ConnectResult) {
		failure = undefined;
		if (res.status === 'connected') {
			password = '';
			keyNote = res.key_note ?? '';
			screen = 'inspect';
			void inspect();
		} else if (res.status === 'unknown_host') {
			fingerprint = res.fingerprint ?? '';
			screen = 'fingerprint';
		} else {
			failure = res;
			screen = 'connect';
		}
	}

	async function connect() {
		if (busy) return;
		busy = true;
		try {
			applyConnect(
				await backend().Connect({ host, user, password, use_keys: useKeys, remember, make_key: makeKey })
			);
		} finally {
			busy = false;
		}
	}

	async function confirmHost() {
		if (busy) return;
		busy = true;
		try {
			applyConnect(await backend().ConfirmHost());
		} finally {
			busy = false;
		}
	}

	async function inspect() {
		busy = true;
		inspection = undefined;
		domainCheck = undefined;
		try {
			inspection = await backend().Inspect();
			// Wynd здесь уже стоит — сразу страница сервера (владелец, 2026-10-24: 7а).
			if (inspection.ok && inspection.report.findings?.some((f) => f.id === 'wynd' || f.id === 'wyndelse')) {
				const p = await backend().ServerPage();
				if (p.ok && p.state.installed && p.state.container) {
					page = p;
					screen = 'server';
				}
			}
		} finally {
			busy = false;
		}
	}

	// Пульт (кадры 7а–7ж2).
	const CHANGES = 'https://github.com/mixeme/wynd/blob/main/docs/CHANGELOG.md';
	let askStop = $state(false);
	let svcBusy = $state('');
	let svcError = $state('');
	let svcLog = $state<LogEntry[]>([]);
	let stopForCopy = $state(true);
	let startAfter = $state(true);
	let buildUpdate = $state(false);
	let returning = $state(false);
	// С какой версии и на какую идёт обновление: окно помнит, пока оно не кончилось.
	let upFrom = $state('');
	let upTo = $state('');

	const st = $derived(page?.state);
	// Кадр 7б2: места не хватает даже на копию базы.
	function noRoom(p: ServerPage | undefined): boolean {
		return Boolean(p && p.state.free_bytes > 0 && p.state.free_bytes < p.copy_bytes + 64 * 1024 * 1024);
	}
	const uplanNoRoom = $derived(noRoom(page));

	function gb(bytes: number): string {
		if (bytes >= 1024 ** 3) return (bytes / 1024 ** 3).toFixed(1).replace('.', ',') + ' ГБ';
		return Math.max(1, Math.round(bytes / 1024 ** 2)) + ' МБ';
	}
	function day(iso: string): string {
		const d = new Date(iso);
		return Number.isNaN(d.getTime())
			? iso
			: d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
	}

	async function refreshPage() {
		page = await backend().ServerPage();
	}

	// «К серверу» после обновления, бэкапа или переезда: состояние сервера
	// могло поменяться (обрыв посреди обновления — кадр 7е), читаем заново.
	async function toServer() {
		screen = 'server';
		svcError = '';
		const p = await backend().ServerPage();
		if (p.ok) page = p;
		else svcError = [p.message, p.advice].filter(Boolean).join('. ');
	}

	function dayTime(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return `${day(iso)} в ${d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}`;
	}

	async function service(action: 'start' | 'stop' | 'restart') {
		if (svcBusy) return;
		svcBusy = action;
		svcError = '';
		try {
			const res = await backend().Service(action);
			page = res.page;
			svcLog = res.log ?? [];
			if (!res.ok) svcError = [res.message, res.advice].filter(Boolean).join('. ');
			askStop = false;
		} finally {
			svcBusy = '';
		}
	}

	function toUpdatePlan() {
		if (!page) return;
		stopForCopy = true;
		startAfter = !page.state.stopped_at;
		// Собран на сервере — и новый собираем, а не молча берём из хранилища (7б4).
		buildUpdate = page.built && page.can_build;
		screen = 'uplan';
	}

	async function startUpdate() {
		if (!page) return;
		upFrom = page.state.update?.from ?? page.state.version ?? '';
		upTo = page.state.update?.to ?? page.latest ?? '';
		screen = 'update';
		applyUpdate(
			await backend().StartUpdate({
				stop_for_copy: stopForCopy || !page.live_copy,
				start_after: page.state.running || startAfter,
				build: buildUpdate
			})
		);
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(applyUpdate), 500);
		}
	}
	function applyUpdate(next: Progress) {
		progress = next;
		if (next.running) return;
		stopPoll();
		if (next.done) {
			screen = 'udone';
			void refreshPage();
		}
	}

	// Бэкап на этот компьютер (8а–8г).
	let bFolder = $state('');
	let bSite = $state<'keep' | 'pause' | 'stop'>('keep');
	let bEncrypt = $state(false);
	let bPass = $state('');
	let bPass2 = $state('');
	let bPlan = $state<LocalBackupPlan | undefined>();

	async function toBackupPlan() {
		if (!page) return;
		const key = 'wynd-workshop-backup:' + (page.state.domain ?? '');
		let saved = '';
		try {
			saved = localStorage.getItem(key) ?? '';
		} catch {
			saved = '';
		}
		bFolder = saved || (await backend().DefaultBackupFolder(page.state.domain ?? 'wynd'));
		bSite = page.live_copy ? 'keep' : 'pause';
		bPass = bPass2 = '';
		screen = 'bplan';
		await planBackup();
	}
	async function planBackup() {
		bPlan = await backend().PlanBackup(bFolder);
		bEncrypt = Boolean(bPlan.encrypted);
	}
	async function chooseBackupFolder() {
		const p = await backend().PickFolder();
		if (p) {
			bFolder = p;
			await planBackup();
		}
	}
	const passOk = $derived(!bEncrypt || (bPass.length > 0 && (bPlan?.encrypted || bPass === bPass2)));
	async function startBackup() {
		if (!page || !bFolder || !passOk) return;
		try {
			localStorage.setItem('wynd-workshop-backup:' + (page.state.domain ?? ''), bFolder);
		} catch {
			/* без памяти папки — спросим её в следующий раз */
		}
		screen = 'backup';
		const apply = (next: Progress) => {
			progress = next;
			if (next.running) return;
			stopPoll();
			if (next.done) {
				screen = 'bdone';
				void refreshPage();
			}
		};
		apply(
			await backend().StartBackup({
				folder: bFolder,
				site: page.state.running ? bSite : 'keep',
				password: bEncrypt ? bPass : ''
			})
		);
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(apply), 500);
		}
	}

	// Переезд (9а–9г).
	let mHost = $state('');
	let mUser = $state('root');
	let mPass = $state('');
	let mUseKeys = $state(false);
	let mMakeKey = $state(false);
	let mFail = $state<ConnectResult | undefined>();
	let mFinger = $state('');
	let mRemoveOld = $state(false);
	let mBusy = $state(false);
	// Незаконченный переезд (9в2): после подключения к новому серверу —
	// продолжить его или отменить.
	let mIntent = $state<'' | 'continue' | 'cancel'>('');

	function toMove() {
		mIntent = '';
		mFail = undefined;
		mPass = '';
		screen = 'mconnect';
	}
	function resumeMove(intent: 'continue' | 'cancel') {
		if (!st?.move) return;
		mIntent = intent;
		mHost = st.move.to;
		mFail = undefined;
		mPass = '';
		screen = 'mconnect';
	}
	async function applyMove(res: ConnectResult) {
		mFail = undefined;
		if (res.status === 'connected') {
			mPass = '';
			keyNote = res.key_note ?? '';
			if (mIntent === 'cancel') {
				await cancelMove(true);
				return;
			}
			screen = 'mplan';
			mplan = undefined;
			mplan = await backend().MovePlan();
			if (mIntent === 'continue' && mplan.ok && mplan.resume) {
				mRemoveOld = Boolean(mplan.remove_old);
				await startMove();
			}
		} else if (res.status === 'unknown_host') {
			mFinger = res.fingerprint ?? '';
			screen = 'mfinger';
		} else {
			mFail = res;
			screen = 'mconnect';
		}
	}
	async function moveConnect() {
		if (mBusy) return;
		mBusy = true;
		try {
			await applyMove(
				await backend().MoveConnect({ host: mHost, user: mUser, password: mPass, use_keys: mUseKeys, remember: false, make_key: mMakeKey })
			);
		} finally {
			mBusy = false;
		}
	}
	async function moveConfirm() {
		if (mBusy) return;
		mBusy = true;
		try {
			await applyMove(await backend().MoveConfirmHost());
		} finally {
			mBusy = false;
		}
	}
	async function startMove() {
		screen = 'move';
		const apply = (next: Progress) => {
			progress = next;
			if (next.running) return;
			stopPoll();
			if (next.done) screen = 'mdone';
		};
		apply(await backend().StartMove(mRemoveOld));
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(apply), 500);
		}
	}
	async function cancelMove(force = false) {
		if (mBusy && !force) return;
		mBusy = true;
		try {
			stopPoll();
			const res = await backend().CancelMove();
			page = res.page;
			svcLog = res.log ?? [];
			if (!res.ok) svcError = [res.message, res.advice].filter(Boolean).join('. ');
			progress = undefined;
			mIntent = '';
			screen = 'server';
		} finally {
			mBusy = false;
		}
	}
	async function adoptNew() {
		page = await backend().MoveAdopt();
		screen = 'server';
	}

	async function returnPrevious() {
		if (returning) return;
		returning = true;
		svcError = '';
		try {
			const res = await backend().ReturnPrevious();
			page = res.page;
			svcLog = res.log ?? [];
			if (res.ok) {
				progress = undefined;
				screen = 'server';
			} else {
				svcError = [res.message, res.advice].filter(Boolean).join('. ');
			}
		} finally {
			returning = false;
		}
	}

	async function checkDomain() {
		if (checkingDomain || !domain.trim()) return;
		checkingDomain = true;
		try {
			domainCheck = await backend().CheckDomain(domain);
			if (domainCheck.level === 'ok') domain = domainCheck.domain;
		} finally {
			checkingDomain = false;
		}
	}

	async function toConnect() {
		stopPoll();
		plan = undefined;
		progress = undefined;
		await backend().Disconnect();
		inspection = undefined;
		domainCheck = undefined;
		failure = undefined;
		screen = 'connect';
	}

	function mark(level: Finding['level']): string {
		return level === 'ok' ? '✓' : '!';
	}

	async function toPlan() {
		if (planning || !domainOk) return;
		planning = true;
		rollbackNote = '';
		try {
			plan = await backend().MakePlan(domain);
			progress = undefined;
			screen = 'plan';
		} finally {
			planning = false;
		}
	}

	// Собрать Wynd на сервере или скачать готовым (окна 3а–3в).
	let choosing = $state(false);
	async function chooseBuild(on: boolean) {
		if (choosing) return;
		choosing = true;
		try {
			const next = await backend().ChooseBuild(on);
			if (next.ok) plan = next;
		} finally {
			choosing = false;
		}
	}

	// Установка идёт в Go сама; окно только спрашивает, как дела.
	let poll: ReturnType<typeof setInterval> | undefined;
	function stopPoll() {
		clearInterval(poll);
		poll = undefined;
	}
	function applyProgress(next: Progress) {
		progress = next;
		if (next.running) return;
		stopPoll();
		if (next.done) screen = 'done';
	}
	async function install() {
		askRollback = false;
		rollbackError = '';
		rollbackLog = [];
		screen = 'install';
		applyProgress(await backend().StartInstall());
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(applyProgress), 500);
		}
	}

	// Данные удаляли — сертификат отзывали: сказать, вышло ли.
	function certNote(cert: 'revoked' | 'failed' | undefined): string {
		if (cert === 'revoked') return ' Сертификат сайта отозван.';
		if (cert === 'failed') {
			return ' Отозвать сертификат сайта не получилось: он удалён вместе с ключом и перестанет действовать сам, когда выйдет его срок.';
		}
		return '';
	}

	// Чужой веб-сервер, за которым стоит Wynd: «nginx», «Apache», «Caddy» или пусто — свой Caddy.
	const proxyName = $derived(
({ nginx: 'nginx', apache: 'Apache', caddy: 'Caddy' } as Record<string, string>)[inspection?.report?.proxy ?? ''] ?? ''
	);
	// Файлы сайта в чужом веб-сервере, которые ставили не мы (кадры 5е, 5е2):
	// Wynd там поставлен не Мастерской; файл не трогаем, а называем.
	const foreignSite = $derived(proxyName ? (inspection?.report?.foreign_site ?? []) : []);

	async function rollback() {
		if (rolling) return;
		rolling = true;
		rollbackError = '';
		// Осмотр после отката тот же веб-сервер найдёт снова; имя берём заранее.
		const stays = proxyName ? `Docker и ${proxyName} остались.` : 'Docker остался.';
		// Кадр 5е2: чужой файл сайта остаётся — сказать, где он.
		const foreign = foreignSite.length;
		const leftFiles = foreign
			? ` Файл сайта ${foreignSite.join(', ')} остался — его ставили не мы.`
			: '';
		// Кадр 5д: что именно помешало, знаем по проверке до удаления и после него.
		const stale = (res: { stale?: string }) => {
			if (!res.stale || !proxyName) return '';
			if (res.stale === 'before') {
				return ` Но ${proxyName} это изменение ещё не принял: в его настройке ошибка, и она была там ещё до удаления — о ней ниже. Когда её исправят, сайт Wynd пропадёт из ${proxyName} сам.`;
			}
			if (res.stale === 'after') {
				return ` Но ${proxyName} это изменение не принял: без файла сайта Wynd его настройка перестала проходить проверку — похоже, на этот файл ссылается чужая настройка. Что не так, сказано ниже.`;
			}
			return ` Но перечитать настройку ${proxyName} не удалось, хотя ошибок в ней нет. Сайт Wynd пропадёт из ${proxyName}, когда его попросят перечитать настройку или перезапустят.`;
		};
		try {
			const res = await backend().Rollback(!dropData);
			if (res.ok) {
				const failed = screen === 'install';
				askRollback = false;
				rollbackLog = [];
				progress = undefined;
				if (res.report) inspection = { ok: true, report: res.report };
				if (res.plan) plan = res.plan;
				if (failed) {
					rollbackNote =
						(dropData
							? 'Установка откачена: Wynd и его данные с сервера убраны. '
							: 'Установка откачена: Wynd с сервера убран, его данные оставлены. ') +
						stays +
						certNote(res.cert) +
						stale(res);
					screen = 'plan';
				} else {
					// Убрали работавший Wynd (окна 2б, 5а): назад к осмотру, сервер уже без него.
					// Сертификат у чужой установки получали не мы — о нём ни слова (5е2).
					rollbackNote =
						(dropData
							? foreign
								? 'Wynd и его данные с сервера убраны. '
								: 'Wynd, его данные и сертификат с сервера убраны. '
							: foreign
								? 'Wynd с сервера убран, его данные оставлены. '
								: 'Wynd с сервера убран, его данные и сертификат оставлены. ') +
						stays +
						leftFiles +
						certNote(res.cert) +
						stale(res);
					screen = 'inspect';
				}
			} else {
				rollbackError = [res.message, res.advice].filter(Boolean).join('. ');
				// Что откат успел и на чём встал — в тех же «подробностях».
				rollbackLog = res.log ?? [];
			}
		} finally {
			rolling = false;
		}
	}

	async function copyRaw() {
		copiedRaw = await backend().CopyText(inspection?.report.raw ?? '');
		setTimeout(() => (copiedRaw = false), 2000);
	}

	async function copyLine(line: string) {
		if (!(await backend().CopyText(line))) return;
		copiedLine = line;
		setTimeout(() => (copiedLine = ''), 2000);
	}

	async function copyLink() {
		if (!progress?.link) return;
		copied = await backend().CopyText(progress.link);
		setTimeout(() => (copied = false), 2000);
	}

	const logText = $derived(
		[...(progress?.log ?? []), ...rollbackLog, ...svcLog].map((e) => ('$ ' + e.command + '\n' + e.output).trimEnd()).join('\n\n')
	);
</script>

{#snippet removeBox(yes: string, site: string)}
	<Notice tone="bad" title="Убрать Wynd с сервера?">
		<div class="ins-dim">
			{#if site}Сайт {site} перестанет открываться.{/if}
			{#if foreignSite.length}
				Уберём контейнер Wynd. Docker и {proxyName} останутся.
			{:else if proxyName}
				Уберём контейнер Wynd и его сайт из {proxyName}. Docker и {proxyName} останутся.
			{:else}
				Уберём контейнеры Wynd и Caddy. Docker останется.
			{/if}
		</div>
		{#if foreignSite.length}
			<!-- Кадр 5е: файл сайта ставили не мы — не трогаем, называем. -->
			<div class="ins-dim">
				Файл сайта в {proxyName} ставили не мы, его не тронем. Пока он есть, {proxyName} отвечает по адресу
				{site || 'сайта'} ошибкой. Уберите его сами:
			</div>
			{#each foreignSite as f (f)}
				<CopyLine text={f} copied={copiedLine === f} oncopy={() => void copyLine(f)} />
			{/each}
		{/if}
		<Checkbox bind:checked={dropData}>
			Удалить данные Wynd — записи, фотографии, учётные записи{foreignSite.length ? '' : ' — и сертификат сайта'}
		</Checkbox>
		<div class="ins-dim">
			{#if foreignSite.length}
				Без галочки папка /opt/wynd останется.
			{:else if proxyName}
				Без галочки папка /opt/wynd и сертификат останутся.
			{:else}
				Без галочки папка /opt/wynd останется как есть.
			{/if}
			Поставите Wynd снова — всё будет на месте.
		</div>
		{#if rollbackError}
			<div class="ins-dim" role="alert">{rollbackError}</div>
			<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
		{/if}
		<FootBar>
			<Button variant="colored" loading={rolling} onclick={() => void rollback()}>{yes}</Button>
			<Button variant="ghost" disabled={rolling} onclick={() => (askRollback = false)}>Отмена</Button>
		</FootBar>
	</Notice>
{/snippet}

<WorkshopTabs
	tabs={[
		{ id: 'servers', label: 'Серверы' },
		{ id: 'archives', label: 'Архивы' }
	]}
	bind:value={tab}
/>

{#if selfOpen && selfUp}
	<!-- Кадры 10б, 10в. -->
	<div class="ins">
		<main class="ins-body">
			{#if selfBad}
				<h2>Эту версию Мастерская не поставит</h2>
				<Notice title="Мастерская {selfUp.version} не подписана ключом Wynd">
					<div class="ins-dim">
						Файл мог испортиться при скачивании — или его подменили. Ставить такую версию опасно: у
						Мастерской есть доступ к вашему серверу. Текущая Мастерская {ownVersion} работает как прежде.
					</div>
				</Notice>
				<FootBar>
					<Button variant="colored" loading={selfBusy} onclick={() => void applySelf()}>Попробовать ещё раз</Button>
					<Button variant="ghost" onclick={() => (selfOpen = false)}>Закрыть</Button>
					<TextButton class="link under ins-push" onclick={() => void backend().ReleasesPage()}>страница выпусков</TextButton>
				</FootBar>
			{:else}
				<h2>Мастерская {selfUp.version}</h2>
				{#if selfUp.notes?.length}
					<p class="ins-sub">Что нового:</p>
					<MarkRows>
						{#each selfUp.notes as n (n)}<MarkRow mark="·">{n}</MarkRow>{/each}
					</MarkRows>
				{/if}
				<MarkRows>
					<MarkRow mark="✓" tone="ok">
						Подписана ключом Wynd <span class="ins-dim">— выложил тот, кто делает Wynd, файл не подменён</span>
					</MarkRow>
					<MarkRow mark="✓" tone="ok">
						Собрана из открытого кода тега workshop-v{selfUp.version} в GitHub Actions
						<TextButton class="link under" onclick={() => void backend().OpenLink(selfUp?.attestation ?? '')}>проверить</TextButton>
					</MarkRow>
				</MarkRows>
				{#if selfError}<Notice tone="bad" alert>{selfError}</Notice>{/if}
				<FootBar>
					<Button
						variant={progress?.running ? 'off' : 'colored'}
						loading={selfBusy}
						onclick={() => void (progress?.running ? undefined : applySelf())}>Обновить и перезапустить</Button
					>
					<Button variant="ghost" onclick={() => (selfOpen = false)}>Не сейчас</Button>
					{#if progress?.running}<span class="ins-dim">Сначала дождитесь окончания</span>{/if}
				</FootBar>
			{/if}
		</main>
	</div>
{:else if tab === 'archives'}
	<Archives />
{:else}
{#if selfUp && !selfHidden}
	<!-- Кадр 10а. -->
	<SelfBar version={selfUp.version ?? ''} onupdate={() => (selfOpen = true)} onhide={() => (selfHidden = true)} />
{/if}
<div class="ins">
	{#if screen !== 'server'}
		<StepNav steps={STEPS} index={stepIndex} bad={stepBad} />
	{/if}

	<main class="ins-body">
		{#if screen === 'connect'}
			<h2>Куда ставим Wynd</h2>
			<p class="ins-sub">Адрес и доступ к серверу вам прислал хостинг после покупки.</p>
			<form
				class="ins-form"
				onsubmit={(e) => {
					e.preventDefault();
					void connect();
				}}
			>
				<Label>Адрес сервера</Label>
				<Input
					mono
					autocomplete="off"
					spellcheck={false}
					placeholder="185.12.34.56"
					bind:value={host}
					onblur={() => void refreshSaved()}
				/>
				<Label>Вход</Label>
				<div class="ins-chips">
					<Chip selected={!useKeys} onclick={() => (useKeys = false)}>Пароль</Chip>
					<Chip selected={useKeys} onclick={() => (useKeys = true)}>Ключ с этого компьютера</Chip>
				</div>
				<div class="ins-pair">
					<div class="ins-user">
						<Label>Пользователь</Label>
						<Input
							mono
							autocomplete="off"
							spellcheck={false}
							bind:value={user}
							onblur={() => void refreshSaved()}
						/>
					</div>
					{#if !useKeys}
						<div class="ins-grow">
							<Label>Пароль от сервера</Label>
							<Input
								type="password"
								autocomplete="off"
								placeholder={saved ? 'сохранён — можно не вводить' : ''}
								bind:value={password}
							/>
						</div>
					{/if}
				</div>
				{#if useKeys}
					<p class="ins-note">Возьмём ключ из папки .ssh вашего профиля — тот же, что у программы ssh.</p>
				{:else}
					<!-- Две независимые галочки, обе выключены (владелец, 2026-10-24). -->
					<Checkbox bind:checked={remember}>
						Запомнить пароль на этом компьютере <span class="ins-dim">— Мастерская будет входить сама</span>
					</Checkbox>
					<Checkbox bind:checked={makeKey}>
						Дальше входить по ключу <span class="ins-dim"
							>— Мастерская создаст ключ на этом компьютере и положит его на сервер</span
						>
					</Checkbox>
				{/if}
				{#if failure}
					<Notice tone="bad" alert title={failure.message}>
						{#if failure.fingerprint}
							<div class="ins-mono">{failure.fingerprint}</div>
						{/if}
						{#if failure.advice}
							<div class="ins-dim">{failure.advice}</div>
						{/if}
					</Notice>
				{/if}
				<FootBar>
					<Button variant="colored" loading={busy} onclick={() => void connect()}>
						Подключиться
					</Button>
					{#if busy}
						<span class="ins-dim">Подключаемся…</span>
					{/if}
				</FootBar>
			</form>
		{:else if screen === 'server' && page && st}
			<!-- Пульт (кадры 7а, 7а2, 7а3, 7е). -->
			{#if keyNote}<Notice tone="note">{keyNote}</Notice>{/if}
			<h2>{st.domain}</h2>
			<p class="ins-sub">
				Сервер {page.host} · Wynd{st.version ? ` ${st.version}` : ''} · {st.running
					? 'работает'
					: st.stopped_at
						? `остановлен вами ${day(st.stopped_at)}`
						: 'не работает'} · свободно {gb(st.free_bytes)}
			</p>
			{#if page.foreign && !st.move}
				<!-- Кадр 7з: только переезд. -->
				{@const f = page.foreign}
				<Notice title="Этот Wynd ставила не Мастерская">
					<div class="ins-dim">
						Он {f.running ? 'работает' : 'запускается'} как {f.kind === 'service'
							? `служба ${f.name}`
							: f.kind === 'container'
								? `контейнер из ${f.name}`
								: `программа ${f.name}`}, данные — в {f.data_dir}. Обновлять такой Wynd, делать его бэкап и убирать
						Мастерская не умеет. Перевезти на другой сервер — умеет: там она поставит Wynd сама и перенесёт данные
						и адрес сайта.
					</div>
				</Notice>
				<MarkRows class="mt-10">
					<MarkRow mark="→">
						Переехать на другой сервер
						{#snippet action()}
							<TextButton class="link under" onclick={toMove}>начать</TextButton>
						{/snippet}
					</MarkRow>
				</MarkRows>
			{:else if st.move}
				<!-- Кадр 9в2. -->
				<Notice title="Переезд на сервер {st.move.to} не закончен">
					<div class="ins-dim">
						Переезд начался {dayTime(st.move.started)}. С тех пор сайт на старом сервере {page.host} остановлен.
					</div>
					<FootBar>
						<Button variant="colored" onclick={() => resumeMove('continue')}>Продолжить переезд</Button>
						<Button variant="ghost" onclick={() => resumeMove('cancel')}>Отменить переезд</Button>
					</FootBar>
				</Notice>
			{:else if st.update}
				<Notice title="Обновление до {st.update.to} не закончено">
					<div class="ins-dim">
						<!-- Момент обрыва Мастерская не знает — время начала обновления (владелец, 2026-10-25). -->
						Обновление началось {dayTime(st.update.started)}. Связь с сервером прервалась{st.update.stopped
							? ', когда Wynd был остановлен,'
							: ''}{st.running ? '' : ' — с тех пор сайт не открывается'}. {st.update.copied
							? 'Копия данных перед обновлением на месте.'
							: 'Копию сделать не успели — данные не менялись.'}
					</div>
					<FootBar>
						<Button variant="colored" onclick={() => void startUpdate()}>Продолжить обновление</Button>
						<Button variant="ghost" loading={returning} onclick={() => void returnPrevious()}>
							Вернуть {st.update.from}
						</Button>
					</FootBar>
				</Notice>
			{:else}
				<div class="ins-svcbar">
					{#if st.running}
						<Button
							variant="ghost"
							loading={svcBusy === 'restart'}
							disabled={Boolean(svcBusy)}
							onclick={() => void service('restart')}>Перезапустить</Button
						>
						<Button variant="ghost" disabled={Boolean(svcBusy)} onclick={() => (askStop = true)}>
							Остановить
						</Button>
					{:else}
						<Button variant="colored" loading={svcBusy === 'start'} onclick={() => void service('start')}>
							Запустить
						</Button>
					{/if}
				</div>
				{#if askStop}
					<Notice title="Остановить сайт?">
						<div class="ins-dim">
							Пока он остановлен, круги не открываются ни у кого, письма с сервера не уходят. Данные
							остаются на месте — «Запустить» вернёт всё как было.
						</div>
						<FootBar>
							<Button variant="colored" loading={svcBusy === 'stop'} onclick={() => void service('stop')}>
								Остановить
							</Button>
							<Button variant="ghost" onclick={() => (askStop = false)}>Отмена</Button>
						</FootBar>
					</Notice>
				{/if}
				{#if page.latest}
					<Notice title="Вышла версия {page.latest}">
						<div class="ins-dim">
							Перед обновлением сохраним копию данных на сервере.
							<TextButton class="link under" onclick={() => void backend().OpenLink(CHANGES)}>
								что нового
							</TextButton>
						</div>
						{#snippet action()}
							<Button variant={st.running ? 'colored' : 'ghost'} onclick={toUpdatePlan}>Обновить</Button>
						{/snippet}
					</Notice>
				{:else}
					<p class="ins-dim">Стоит самая свежая версия.</p>
				{/if}
				<MarkRows>
					<MarkRow mark="↓">
						Бэкап на этот компьютер
						{#snippet action()}
							<TextButton class="link under" onclick={() => void toBackupPlan()}>сделать</TextButton>
						{/snippet}
					</MarkRow>
					<MarkRow mark="→">
						Переехать на другой сервер
						{#snippet action()}
							<TextButton class="link under" onclick={toMove}>начать</TextButton>
						{/snippet}
					</MarkRow>
				</MarkRows>
			{/if}
			{#if svcError}
				<Notice tone="bad" alert>
					{svcError}
					<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
				</Notice>
			{/if}
			{#if askRollback}
				{@render removeBox('Да, убрать', st.domain ?? '')}
			{/if}
			<FootBar>
				<Button variant="ghost" onclick={() => void toConnect()}>Другой сервер</Button>
				{#if selfUp && selfHidden}
					<span class="ins-dim ins-push"
						>Мастерская {ownVersion} · есть {selfUp.version} —
						<TextButton class="link under" onclick={() => (selfOpen = true)}>обновить</TextButton></span
					>
				{/if}
				{#if !askRollback && !st.move && !page.foreign}
					<TextButton class="link under {selfUp && selfHidden ? '' : 'ins-push'}" onclick={() => (askRollback = true)}>
						Убрать Wynd
					</TextButton>
				{/if}
			</FootBar>
			{#if showLog}
				<CommandLog text={logText} />
			{/if}
		{:else if screen === 'uplan' && page && st && uplanNoRoom}
			<!-- Кадр 7б2. -->
			<h2>Пока обновить нельзя</h2>
			<Notice title="На сервере мало места для копии">
				<div class="ins-dim">
					Перед обновлением сохраняем копию данных: без неё при неудаче их нечем вернуть. Копии нужно
					{gb(page.copy_bytes)}, свободно {gb(st.free_bytes)}. Освободите место на сервере и нажмите
					«Проверить снова».
				</div>
			</Notice>
			<FootBar>
				<Button variant="colored" onclick={() => void refreshPage()}>Проверить снова</Button>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
			</FootBar>
		{:else if screen === 'uplan' && page && st}
			<!-- Кадры 7б, 7б3, 7б4. -->
			{@const get = buildUpdate ? `Соберём Wynd ${page.latest} из исходного кода` : `Скачаем Wynd ${page.latest}`}
			{@const stopLive = st.running && !(stopForCopy || !page.live_copy)}
			<h2>Обновим до {page.latest}</h2>
			<MarkRows>
				<MarkRow mark="1">
					{get}{#if st.running}&nbsp;<span class="ins-dim">— сайт пока работает</span>{/if}
				</MarkRow>
				{#if st.running && !stopLive}
					<MarkRow mark="2">
						Остановим Wynd <span class="ins-dim">— с этой минуты сайт не открывается</span>
					</MarkRow>
				{/if}
				<MarkRow mark={st.running && !stopLive ? 3 : 2}>
					Сохраним копию данных на сервере
					<span class="ins-dim"
						>— займёт {gb(page.copy_bytes)}, свободно {gb(st.free_bytes)}. Фото и видео обновление не
						меняет: в копии они те же файлы и места не занимают</span
					>
				</MarkRow>
				{#if stopLive}
					<MarkRow mark="3">
						Остановим Wynd <span class="ins-dim">— с этой минуты сайт не открывается</span>
					</MarkRow>
				{/if}
				<MarkRow mark={st.running ? 4 : 3}>
					{st.running || startAfter ? `Запустим ${page.latest} и проверим сайт` : `Поставим ${page.latest}`}
				</MarkRow>
			</MarkRows>
			{#if st.running}
				<Checkbox bind:checked={stopForCopy} disabled={!page.live_copy}>
					Остановить сайт на время копии
					<span class="ins-dim">
						{#if !page.live_copy}
							— Wynd {st.version} копию без остановки не умеет.
						{/if}
						Пока идёт копия, никто ничего не напишет — при возврате к {st.version} ничего не пропадёт.
						Без галочки сайт закрыт только на запуск новой версии, но то, что напишут во время копии,
						при возврате пропадёт.
					</span>
				</Checkbox>
			{:else}
				<Label>После обновления</Label>
				<Radio name="after" value={true} bind:group={startAfter}>Запустить новую версию</Radio>
				<Radio name="after" value={false} bind:group={startAfter}>
					Оставить сайт остановленным
					{#if st.stopped_at}<span class="ins-dim">— его остановили вы {day(st.stopped_at)}</span>{/if}
				</Radio>
			{/if}
			<p class="ins-sub mt">Не тронем: Docker, {proxyName || 'Caddy'}, сертификат сайта.</p>
			{#if page.can_build}
				<p class="ins-sub">
					{buildUpdate ? 'Сервер соберёт Wynd сам.' : 'Сервер скачает Wynd готовым.'}
					<TextButton class="link under" onclick={() => (buildUpdate = !buildUpdate)}>
						{buildUpdate ? 'Скачать готовый' : 'Собрать самому'}
					</TextButton>
				</p>
			{/if}
			<FootBar>
				<Button variant="colored" onclick={() => void startUpdate()}>Обновить</Button>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
				{#if buildUpdate}<span class="ins-dim">около 10 минут</span>{/if}
			</FootBar>
		{:else if screen === 'update' && progress?.failure}
			<!-- Кадр 7д: после остановки — вернуть прежнюю; раньше сайт работает, только повторить. -->
			{@const stopped = ['start', 'check'].includes(progress.failure.step)}
			<h2>{progress.failure.message}</h2>
			{#if progress.failure.advice}<Notice>{progress.failure.advice}</Notice>{/if}
			{#if stopped}
				<p class="ins-sub mt">
					«Вернуть {upFrom}» запустит прежнюю версию с данными из копии — сайт заработает как до
					обновления.
				</p>
			{/if}
			{#if svcError}<Notice tone="bad" alert>{svcError}</Notice>{/if}
			<FootBar>
				{#if stopped}
					<Button variant="colored" loading={returning} onclick={() => void returnPrevious()}>
						Вернуть {upFrom}
					</Button>
					<Button variant="ghost" disabled={returning} onclick={() => void startUpdate()}>Повторить</Button>
				{:else}
					<Button variant="colored" onclick={() => void startUpdate()}>Повторить</Button>
					<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
				{/if}
				<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}>подробности</TextButton>
			</FootBar>
			{#if showLog}
				<CommandLog text={logText} />
			{/if}
		{:else if screen === 'update'}
			<h2>Обновляем</h2>
			<StepList steps={progress?.steps ?? []} />
			<FootBar>
				<TextButton class="link under" onclick={() => (showLog = !showLog)}>
					показать, что выполняется
				</TextButton>
				<span class="ins-dim">Окно можно свернуть</span>
			</FootBar>
			{#if showLog}
				<CommandLog text={logText} />
			{/if}
		{:else if screen === 'udone'}
			<!-- Кадр 7г. -->
			<h2>Wynd обновлён</h2>
			<p class="ins-sub">
				{page?.state.running
					? `Проверка прошла: работает ${upTo}, сайт открывается.`
					: `Wynd ${upTo} поставлен, сайт остановлен.`}
			</p>
			<Notice>
				Копия данных до обновления лежит на сервере в папке /opt/wynd/backups/{upFrom}{page
					? ` — ${gb(page.copy_bytes)}`
					: ''}.
			</Notice>
			<FootBar>
				{#if page?.state.running}
					<Button variant="colored" onclick={() => void backend().OpenLink(`https://${page?.state.domain}`)}>
						Открыть {page?.state.domain}
					</Button>
				{/if}
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
			</FootBar>
		{:else if screen === 'bplan' && page && st}
			<!-- Кадры 8а, 8а2. -->
			<h2>Бэкап на этот компьютер</h2>
			<p class="ins-sub">
				Вся папка Wynd на сервере — из неё Wynd можно поставить заново: записи, фото и видео, учётные
				записи, настройки и сертификат сайта.
			</p>
			<Label>Папка</Label>
			<FieldLine>
				<Input mono class="ins-grow" bind:value={bFolder} onblur={() => void planBackup()} />
				<Button variant="ghost" onclick={() => void chooseBackupFolder()}>Выбрать</Button>
			</FieldLine>
			{#if bPlan?.ok}
				<MarkRow mark="↓">
					Скачаем {gb(bPlan.fetch)}
					<span class="ins-dim"
						>{bPlan.prev
							? `— в папке уже есть бэкап от ${day(bPlan.prev)}, берём только новое. Всего ${gb(bPlan.total)}`
							: ''}</span
					>
				</MarkRow>
			{:else if bPlan}
				<Notice tone="bad" alert>{bPlan.message}</Notice>
			{/if}
			{#if st.running}
				<Label>Сайт</Label>
				<Radio name="bsite" value="keep" bind:group={bSite} disabled={!page.live_copy}>
					Работает, пока идёт бэкап{#if !page.live_copy}&nbsp;<span class="ins-dim">— Wynd {st.version} так не умеет</span>{/if}
				</Radio>
				<Radio name="bsite" value="pause" bind:group={bSite}>Остановить на время бэкапа и запустить снова</Radio>
				<Radio name="bsite" value="stop" bind:group={bSite}>
					Остановить и не запускать <span class="ins-dim">— сервер переезжает или закрывается</span>
				</Radio>
			{/if}
			<Checkbox bind:checked={bEncrypt} disabled={bPlan?.encrypted}>Зашифровать паролем</Checkbox>
			{#if bEncrypt}
				<div class="ins-pair">
					<div class="ins-grow">
						<Label>Пароль</Label><Input type="password" autocomplete="off" bind:value={bPass} />
					</div>
					{#if !bPlan?.encrypted}
						<div class="ins-grow">
							<Label>Ещё раз</Label><Input type="password" autocomplete="off" bind:value={bPass2} />
						</div>
					{/if}
				</div>
				<p class="ins-dim">
					{bPlan?.encrypted
						? 'В этой папке бэкап с паролем — нужен тот же пароль, иначе бэкап скачается целиком заново в другую папку.'
						: 'Без пароля бэкап не открыть никак — запишите его.'}
				</p>
			{/if}
			<FootBar>
				<Button
					variant={bFolder && passOk && bPlan?.ok && (bSite !== 'keep' || page.live_copy || !st.running)
						? 'colored'
						: 'off'}
					onclick={() => void startBackup()}>Сделать бэкап</Button
				>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
			</FootBar>
		{:else if screen === 'backup' && progress?.failure}
			<!-- Кадр 8г. -->
			<h2>{progress.failure.message}</h2>
			{#if progress.failure.advice}<Notice>{progress.failure.advice}</Notice>{/if}
			{#if progress.failure.step === 'fetch' && bPlan?.prev}
				<p class="ins-sub mt">Прошлый бэкап в папке не тронут, пока новый не скачан целиком.</p>
			{/if}
			<FootBar>
				<Button variant="colored" onclick={() => void startBackup()}>Повторить</Button>
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
				<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}>подробности</TextButton>
			</FootBar>
			{#if showLog}
				<CommandLog text={logText} />
			{/if}
		{:else if screen === 'backup'}
			<!-- Кадр 8б. -->
			<h2>Делаем бэкап</h2>
			<StepList steps={progress?.steps ?? []} />
			<FootBar>
				<TextButton class="link under" onclick={() => (showLog = !showLog)}>
					показать, что выполняется
				</TextButton>
				<span class="ins-dim">Окно можно свернуть</span>
			</FootBar>
			{#if showLog}
				<CommandLog text={logText} />
			{/if}
		{:else if screen === 'bdone'}
			<!-- Кадр 8в. -->
			<h2>Бэкап готов</h2>
			<p class="ins-sub">В папке {bFolder}.</p>
			<Notice>
				{bEncrypt
					? 'Бэкап зашифрован. Без пароля его не открыть.'
					: 'В бэкапе — все записи и ключи сервера. Храните его так же бережно, как пароли: кто получит папку, прочтёт все круги.'}
			</Notice>
			<FootBar>
				<Button variant="colored" onclick={() => void backend().ShowInFolder(bFolder + '\\wynd-backup.json')}>
					Открыть папку
				</Button>
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
			</FootBar>
		{:else if screen === 'mconnect' && st}
			<!-- Кадр 9а. -->
			<h2>Куда переезжает {st.domain}</h2>
			<p class="ins-sub">Адрес сайта останется прежним — люди откроют Wynd как раньше, с тех же телефонов.</p>
			<form
				class="ins-form"
				onsubmit={(e) => {
					e.preventDefault();
					void moveConnect();
				}}
			>
				<Label>Адрес нового сервера</Label>
				<Input mono autocomplete="off" spellcheck={false} placeholder="203.0.113.7" bind:value={mHost} />
				<Label>Вход</Label>
				<div class="ins-chips">
					<Chip selected={!mUseKeys} onclick={() => (mUseKeys = false)}>Пароль</Chip>
					<Chip selected={mUseKeys} onclick={() => (mUseKeys = true)}>Ключ с этого компьютера</Chip>
				</div>
				<div class="ins-pair">
					<div class="ins-user">
						<Label>Пользователь</Label>
						<Input mono autocomplete="off" spellcheck={false} bind:value={mUser} />
					</div>
					{#if !mUseKeys}
						<div class="ins-grow">
							<Label>Пароль от сервера</Label>
							<Input type="password" autocomplete="off" bind:value={mPass} />
						</div>
					{/if}
				</div>
				{#if !mUseKeys}
					<Checkbox bind:checked={mMakeKey}>
						Дальше входить по ключу <span class="ins-dim"
							>— Мастерская создаст ключ на этом компьютере и положит его на сервер</span
						>
					</Checkbox>
				{/if}
				{#if mFail}
					<Notice tone="bad" alert title={mFail.message} text={mFail.advice} />
				{/if}
				<FootBar>
					<Button variant="colored" loading={mBusy} onclick={() => void moveConnect()}>Подключиться</Button>
					<Button
						variant="ghost"
						onclick={() => {
							mIntent = '';
							screen = 'server';
						}}>Назад</Button
					>
				</FootBar>
			</form>
		{:else if screen === 'mfinger'}
			<h2>Это ваш новый сервер?</h2>
			<p class="ins-sub">
				Подключаемся к {mHost.trim()} впервые. Сервер назвал свой отпечаток — сверьте его с письмом хостинга
				или панелью, если там он есть.
			</p>
			<Notice mono>{mFinger}</Notice>
			<FootBar>
				<Button variant="colored" loading={mBusy} onclick={() => void moveConfirm()}>Да, подключиться</Button>
				<Button variant="ghost" disabled={mBusy} onclick={() => (screen = 'mconnect')}>Отмена</Button>
			</FootBar>
		{:else if screen === 'mplan' && !mplan}
			<h2>Что сделаем</h2>
			<p class="ins-dim">Осматриваем новый сервер…</p>
		{:else if screen === 'mplan' && mplan && !mplan.ok}
			<h2>Пока переехать нельзя</h2>
			<Notice alert title={mplan.message} text={mplan.advice} />
			<FootBar>
				<Button variant="ghost" onclick={() => (screen = 'mconnect')}>Другой сервер</Button>
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
			</FootBar>
		{:else if screen === 'mplan' && mplan}
			<!-- Кадры 9б и 9б2 (на старом Wynd не со своим Caddy — переезжают одни данные). -->
			{@const dataOnly = mplan.data_only}
			{@const oldProxy = ({ nginx: 'nginx', apache: 'Apache', caddy: 'Caddy' } as Record<string, string>)[mplan.old_proxy ?? ''] ?? ''}
			<h2>Что сделаем</h2>
			<MarkRows>
				<MarkRow mark="1">Поставим Wynd {mplan.version}{dataOnly ? ' и Caddy' : ''} на новый сервер <span class="ins-dim">— пока без данных и не запуская</span></MarkRow>
				<MarkRow mark="2">Остановим сайт на старом <span class="ins-dim">— с этой минуты он не открывается</span></MarkRow>
				<MarkRow mark="3">Перенесём {dataOnly ? 'данные' : 'папку'} Wynd на новый сервер <span class="ins-dim">— {gb(mplan.size)}, напрямую со старого, мимо этого компьютера</span></MarkRow>
				<MarkRow mark="4">Вы смените адрес <span class="ins-dim">— у записи A для {mplan.domain} поставите {mplan.new_ip}</span></MarkRow>
				<MarkRow mark="5">{dataOnly ? 'Запустим Wynd на новом, получим сертификат сайта и проверим сайт' : 'Запустим Wynd на новом и проверим сайт'}</MarkRow>
			</MarkRows>
			{#if dataOnly && oldProxy}
				<p class="ins-sub mt">Не тронем на старом: {oldProxy} и его сертификат.</p>
			{/if}
			{#if !mplan.foreign}
				<Label>Старый сервер</Label>
				<Radio name="mold" value={false} bind:group={mRemoveOld}>
					Оставить Wynd остановленным <span class="ins-dim">— убрать потом, когда убедитесь, что всё на месте</span>
				</Radio>
				<Radio name="mold" value={true} bind:group={mRemoveOld}>
					Убрать Wynd после переезда <span class="ins-dim">— со старого сервера уйдут его данные{dataOnly ? '' : '; сертификат переедет на новый'}</span>
				</Radio>
			{/if}
			<FootBar>
				<Button variant="colored" onclick={() => void startMove()}>Переехать</Button>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
			</FootBar>
		{:else if screen === 'move' && progress?.failure}
			<h2>{progress.failure.message}</h2>
			{#if progress.failure.advice}<Notice>{progress.failure.advice}</Notice>{/if}
			<FootBar>
				<Button variant="colored" onclick={() => void startMove()}>Повторить</Button>
				<Button variant="ghost" loading={mBusy} onclick={() => void cancelMove()}>Отменить переезд</Button>
				<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}>подробности</TextButton>
			</FootBar>
			{#if showLog}<CommandLog text={logText} />{/if}
		{:else if screen === 'move'}
			<!-- Кадр 9в. -->
			{@const waitingDNS = progress?.steps?.some((s) => s.id === 'dns' && s.status === 'running')}
			<h2>Переезжаем</h2>
			<StepList steps={progress?.steps ?? []} />
			{#if waitingDNS && mplan}
				<Notice>
					<div class="ins-dim">В панели, где покупали адрес {mplan.domain}, поменяйте у записи A адрес сервера на этот:</div>
					<CopyLine text={mplan.new_ip} copied={copiedLine === mplan.new_ip} oncopy={() => void copyLine(mplan?.new_ip ?? '')} />
					<div class="ins-dim">Мастерская проверяет сама каждую минуту — окно можно свернуть.</div>
				</Notice>
				<FootBar>
					<Button variant="ghost" onclick={() => void backend().MoveCheckNow()}>Проверить сейчас</Button>
					<TextButton class="link under ins-push" onclick={() => void cancelMove()}>Отменить переезд</TextButton>
				</FootBar>
			{:else}
				<FootBar>
					<TextButton class="link under" onclick={() => (showLog = !showLog)}>показать, что выполняется</TextButton>
					<span class="ins-dim">Окно можно свернуть</span>
				</FootBar>
			{/if}
			{#if showLog}<CommandLog text={logText} />{/if}
		{:else if screen === 'mdone' && mplan}
			<!-- Кадр 9г. -->
			<h2>Wynd на новом сервере</h2>
			<p class="ins-sub">Проверка прошла: {mplan.domain} открывается с сервера {mplan.new_ip}.</p>
			<Notice>
				{mRemoveOld
					? `Wynd со старого сервера ${mplan.old_ip} убран.`
					: `Старый сервер ${mplan.old_ip}: Wynd остановлен, данные на месте.${mplan.foreign
							? ''
							: ' Когда убедитесь, что на новом всё есть, уберите его со страницы старого сервера.'}`}
			</Notice>
			<FootBar>
				<Button variant="colored" onclick={() => void backend().OpenLink(`https://${mplan?.domain}`)}>
					Открыть {mplan.domain}
				</Button>
				<Button variant="ghost" onclick={() => void adoptNew()}>К серверу</Button>
			</FootBar>
		{:else if screen === 'fingerprint'}
			<h2>Это ваш сервер?</h2>
			<p class="ins-sub">
				Подключаемся к {host.trim()} впервые. Сервер назвал свой отпечаток — сверьте его с письмом
				хостинга или панелью, если там он есть.
			</p>
			<Notice mono>{fingerprint}</Notice>
			<p class="ins-sub mt">
				Если отпечатка негде взять — это нормально для нового сервера. Мы запомним его, и при
				следующем подключении подмену заметим.
			</p>
			<FootBar>
				<Button variant="colored" loading={busy} onclick={() => void confirmHost()}>
					Да, подключиться
				</Button>
				<Button variant="ghost" disabled={busy} onclick={() => void toConnect()}>Отмена</Button>
			</FootBar>
		{:else if screen === 'plan' && plan && !plan.ok}
			{#if rollbackNote}
				<Notice tone="note">{rollbackNote}</Notice>
			{/if}
			<h2>Пока ставить нельзя</h2>
			<Notice alert title={plan.message} text={plan.advice} />
			<FootBar>
				<Button variant="ghost" onclick={() => (screen = 'inspect')}>Назад</Button>
			</FootBar>
		{:else if screen === 'plan' && plan}
			<h2>Что сделаем</h2>
			<p class="ins-sub">До кнопки «Установить» на сервере ничего не меняется.</p>
			{#if rollbackNote}
				<Notice>{rollbackNote}</Notice>
			{/if}
			<div class="ins-sec">Поставим</div>
			{#each plan.install ?? [] as item (item)}
				<MarkRow mark="+" tone="ok">{item}</MarkRow>
			{/each}
			{#if plan.change?.length}
				<div class="ins-sec">Изменим</div>
				{#each plan.change as item (item)}
					<MarkRow mark="~" tone="note">{item}</MarkRow>
				{/each}
			{/if}
			{#if plan.keep?.length}
				<div class="ins-sec">Не тронем</div>
				{#each plan.keep as item (item)}
					<MarkRow mark="·">{item}</MarkRow>
				{/each}
			{/if}
			{#if plan.unpublished}
				<p class="ins-sub">Эту версию ещё нельзя скачать готовой — сервер соберёт Wynd сам.</p>
			{:else if plan.canBuild}
				<p class="ins-sub">
					{plan.build ? 'Сервер соберёт Wynd сам.' : 'Сервер скачает Wynd готовым.'}
					<TextButton class="link under" onclick={() => void chooseBuild(!plan?.build)}>
						{plan.build ? 'Скачать готовый' : 'Собрать самому'}
					</TextButton>
				</p>
			{/if}
			<FootBar>
				<Button variant="colored" onclick={() => void install()}>Установить</Button>
				<Button variant="ghost" onclick={() => (screen = 'inspect')}>Назад</Button>
				<span class="ins-dim">{plan.duration}</span>
			</FootBar>
		{:else if screen === 'install' && progress?.failure}
			<h2>{progress.failure.message}</h2>
			<p class="ins-sub">{progress.failure.advice}</p>
			{#if progress.steps?.some((s) => s.status === 'done')}
				<Notice>
					Сделанное раньше осталось на месте — повтор начнётся с этого шага.
				</Notice>
			{/if}
			{#if askRollback}
				{@render removeBox('Да, откатить', '')}
			{:else}
				<FootBar>
					<Button variant="colored" onclick={() => void install()}>Повторить</Button>
					<Button variant="ghost" onclick={() => (askRollback = true)}>Откатить установку</Button>
					<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
				</FootBar>
			{/if}
			{#if showLog}
				<CommandLog text={logText} />
			{/if}
		{:else if screen === 'install'}
			<h2>Ставим</h2>
			<StepList steps={progress?.steps ?? []} />
			<FootBar>
				<TextButton class="link under" onclick={() => (showLog = !showLog)}>
					показать, что выполняется
				</TextButton>
				<span class="ins-dim">Окно можно свернуть</span>
			</FootBar>
			{#if showLog}
				<CommandLog text={logText} />
			{/if}
		{:else if screen === 'done' && progress}
			<h2>Wynd работает</h2>
			<p class="ins-sub">Проверка прошла: сайт открывается, сертификат действует.</p>
			{#if askRollback}
				{@render removeBox('Да, убрать', plan?.domain ?? '')}
				{#if showLog && rollbackLog.length}
					<CommandLog text={logText} />
				{/if}
			{:else if progress.link}
				<Label>Ссылка первого запуска — для вас, одна</Label>
				<Notice mono>{progress.link}</Notice>
				<p class="ins-sub mt">
					По ней вы станете администратором сервера и создадите первый круг. Никому её не
					передавайте.
				</p>
				<FootBar>
					<Button variant="colored" onclick={() => void backend().OpenLink(progress?.link ?? '')}>
						Открыть в браузере
					</Button>
					<Button variant="ghost" onclick={() => void copyLink()}>
						{copied ? 'Скопировано' : 'Скопировать'}
					</Button>
					<TextButton class="link under ins-push" onclick={() => (askRollback = true)}>
						Убрать Wynd
					</TextButton>
				</FootBar>
			{:else}
				<Notice>
					Этот сервер уже настроен: администратор у него есть, ссылки первого запуска нет.
					Откройте сайт и войдите как обычно.
				</Notice>
				<FootBar>
					<Button variant="colored" onclick={() => void backend().OpenLink(progress?.site ?? '')}>
						Открыть {plan?.domain}
					</Button>
					<TextButton class="link under ins-push" onclick={() => (askRollback = true)}>
						Убрать Wynd
					</TextButton>
				</FootBar>
			{/if}
		{:else if !inspection}
			<h2>Что на сервере</h2>
			<p class="ins-sub">Только смотрим — ничего не меняем.</p>
			<p class="ins-dim">Осматриваем…</p>
		{:else if !inspection.ok}
			<h2>Осмотр не удался</h2>
			<Notice tone="bad" alert title={inspection.message} text={inspection.advice} />
			<FootBar>
				<Button variant="colored" onclick={() => void toConnect()}>Подключиться ещё раз</Button>
			</FootBar>
		{:else if blocks.length}
			<!-- Итог удаления не теряется, когда новый осмотр нашёл помеху (кадр 5д). -->
			{#if rollbackNote}
				<Notice tone="note">{rollbackNote}</Notice>
			{/if}
			<h2>Пока ставить нельзя</h2>
			<p class="ins-sub">
				{blocks.length === 1 ? 'Мешает одно:' : 'Мешает вот что:'}
			</p>
			{#each blocks as block (block.id)}
				<Notice title={block.text} text={block.advice}>
					{#if block.copy}
						<CopyLine text={block.copy} copied={copiedLine === block.copy} oncopy={() => void copyLine(block.copy ?? '')} />
					{/if}
					{#if block.after}
						<div class="ins-dim">{block.after}</div>
					{/if}
				</Notice>
			{/each}
			<div class="ins-more">
				<TextButton class="link under" onclick={() => (showRaw = true)}>
					подробнее, что проверили
				</TextButton>
			</div>
			<FootBar>
				{#if blocks.every((b) => b.final)}
					<Button variant="colored" disabled={busy} onclick={() => void toConnect()}>
						Другой сервер
					</Button>
					<Button variant="ghost" loading={busy} onclick={() => void inspect()}>
						Проверить снова
					</Button>
				{:else}
					<Button variant="colored" loading={busy} onclick={() => void inspect()}>
						Проверить снова
					</Button>
					<Button variant="ghost" disabled={busy} onclick={() => void toConnect()}>
						Другой сервер
					</Button>
				{/if}
			</FootBar>
		{:else}
			<h2>Что на сервере</h2>
			<p class="ins-sub">Только смотрим — ничего не меняем.</p>
			{#if rollbackNote}
				<Notice>{rollbackNote}</Notice>
			{/if}
			<MarkRows>
				{#each findings as f (f.id)}
					<MarkRow mark={mark(f.level)} tone={f.level}>
						{f.text}{#if f.plan}&nbsp;— <span class="ins-dim">{f.plan}{f.id === 'wynd' ? '.' : ''}</span>{/if}
						{#if f.id === 'wynd' && !askRollback}
							<TextButton class="link under" onclick={() => (askRollback = true)}>
								Убрать Wynd с сервера
							</TextButton>
						{/if}
					</MarkRow>
				{/each}
			</MarkRows>
			{#if askRollback}
				{@render removeBox('Да, убрать', domainOk ? domain : '')}
				{#if showLog && rollbackLog.length}
					<CommandLog text={logText} />
				{/if}
			{/if}
			<Label>Адрес, по которому будут заходить</Label>
			<form
				class="ins-domain"
				onsubmit={(e) => {
					e.preventDefault();
					void checkDomain();
				}}
			>
				<Input
					mono
					class="ins-grow"
					autocomplete="off"
					spellcheck={false}
					placeholder="family.example.ru"
					bind:value={domain}
					oninput={() => (domainCheck = undefined)}
				/>
				{#if domainOk}
					<span class="ins-mark ok wide">✓ {domainCheck?.text}</span>
				{:else}
					<Button variant="ghost" loading={checkingDomain} onclick={() => void checkDomain()}>
						{domainCheck ? 'Проверить снова' : 'Проверить'}
					</Button>
				{/if}
			</form>
			{#if domainCheck && !domainOk}
				<Notice alert title={domainCheck.text} text={domainCheck.advice} />
			{/if}
			<FootBar>
				<Button
					variant={domainOk ? 'colored' : 'off'}
					loading={planning}
					onclick={() => void toPlan()}
				>
					Дальше — план
				</Button>
				<Button variant="ghost" onclick={() => void toConnect()}>Другой сервер</Button>
				{#if !domainOk}
					<span class="ins-dim">Сначала укажите адрес сайта</span>
				{/if}
				<TextButton class="link under" onclick={() => (showRaw = true)}>
					подробнее, что проверили
				</TextButton>
			</FootBar>
		{/if}
	</main>
	{#if showRaw && inspection}
		<RawSheet parts={rawParts} copied={copiedRaw} oncopy={() => void copyRaw()} onclose={() => (showRaw = false)} />
	{/if}
</div>
{/if}
