// Контроллер панели Мастерской (план 60): состояние и потоки операций —
// подключение, осмотр, план, установка и откат, пульт, обновление, бэкап,
// переезд. Разметка остаётся в App.svelte и читает поля контроллера.
// Поведение то же, что было в script-блоке App.svelte до выноса.
import { onDestroy, onMount, untrack } from 'svelte';
import {
	backend,
	type Backend,
	type ConnectResult,
	type DomainCheck,
	type Finding,
	type InspectResult,
	type LocalBackupPlan,
	type LogEntry,
	type MovePlan,
	type Plan,
	type ProfileRequest,
	type ProfileViewState,
	type Progress,
	type ServerPage,
	type ServerProfile
} from '../api';
import { nextLogChunk } from './buildlog';
import { noRoom } from './display';
import { rawSections } from '../raw';
import { removalNote } from './rollback';

export type Screen =
	| 'offline'
	| 'new'
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

export interface PanelInput {
	profile: () => ServerProfile;
	profiles: () => ServerProfile[];
	request: () => ProfileRequest | undefined;
	reconnect: () => number;
	onconsumed: () => void;
	onstate: (s: ProfileViewState) => void;
	onprofileschange: () => Promise<void>;
	onselect: (id: string) => void;
	onmoved: (id: string) => Promise<void>;
}

// Порция догрузки начала журнала — nextLogChunk в ./buildlog (C71).

export class Panel {
	readonly input: PanelInput;
	readonly api: Backend;

	// --- Подключение и осмотр (окна 1–2а) ---
	operationError = $state('');
	connection = $state<'disconnected' | 'connecting' | 'connected' | 'fingerprint' | 'password_required' | 'error'>(
		'disconnected'
	);
	pendingAuto = false;
	seenRequest = 0;
	seenReconnect = 0;
	password = $state('');
	sessionRemember = $state(false);
	makeKey = $state(false);
	keyNote = $state('');
	saved = $state(false);
	busy = $state(false);
	failure = $state<ConnectResult | undefined>();
	fingerprint = $state('');
	previousFingerprint = $state('');
	changedFingerprint = $state(false);
	inspection = $state<InspectResult | undefined>();
	screen = $state<Screen>('offline');

	// --- Осмотр: помехи и подробности ---
	showRaw = $state(false);
	copiedRaw = $state(false);
	domain = $state('');
	// Каталог установки: нужен только при установке (план «другой каталог»).
	// Значение по умолчанию — сохранённый каталог сервера, иначе /opt/wynd.
	installRoot = $state('');
	domainCheck = $state<DomainCheck | undefined>();
	checkingDomain = $state(false);

	findings = $derived<Finding[]>(this.inspection?.report.findings ?? []);
	blocks = $derived(this.findings.filter((f) => f.level === 'block'));
	// Сертификаты, продление которых сломалось (Stale): находка в списке, а
	// совет с командой — отдельным блоком под списком.
	staleCerts = $derived(this.findings.filter((f) => f.id === 'certstale' && f.advice && f.copy));
	rawParts = $derived(
		this.inspection
			? rawSections(
					this.inspection.report.raw,
					this.blocks.map((b) => b.id)
				)
			: []
	);

	// --- План, установка и откат (окна 3–5е2) ---
	plan = $state<Plan | undefined>();
	planning = $state(false);
	progress = $state<Progress | undefined>();
	showLog = $state(false);
	askRollback = $state(false);
	dropData = $state(false);
	rolling = $state(false);
	rollbackNote = $state('');
	rollbackError = $state('');
	rollbackLog = $state<LogEntry[]>([]);
	copied = $state(false);
	copiedLine = $state('');

	// --- Пульт (кадры 7а–7е) ---
	page = $state<ServerPage | undefined>();
	askStop = $state(false);
	svcBusy = $state('');
	svcError = $state('');
	svcLog = $state<LogEntry[]>([]);
	// С какой версии и на какую идёт обновление: окно помнит, пока оно не кончилось.
	upFrom = $state('');
	upTo = $state('');

	st = $derived(this.page?.state);
	uplanNoRoom = $derived(noRoom(this.page));

	// --- Обновление (7б–7г) ---
	stopForCopy = $state(true);
	startAfter = $state(true);
	buildUpdate = $state(false);
	returning = $state(false);

	// --- Бэкап на этот компьютер (8а–8г) ---
	bFolder = $state('');
	bSite = $state<'keep' | 'pause' | 'stop'>('keep');
	bEncrypt = $state(false);
	bPass = $state('');
	bPass2 = $state('');
	bPlan = $state<LocalBackupPlan | undefined>();

	// --- Переезд (9а–9г) ---
	targetID = $state('');
	completedTarget = $state('');
	mPass = $state('');
	mFail = $state<ConnectResult | undefined>();
	mFinger = $state('');
	mRemoveOld = $state(false);
	mBusy = $state(false);
	// Незаконченный переезд (9в2): после подключения к новому серверу —
	// продолжить его или отменить.
	mIntent = $state<'' | 'continue' | 'cancel'>('');
	mplan = $state<MovePlan | undefined>();

	// --- Общее отображение ---
	updating = $derived(['uplan', 'update', 'udone'].includes(this.screen));
	backingUp = $derived(['bplan', 'backup', 'bdone'].includes(this.screen));
	moving = $derived(['mconnect', 'mfinger', 'mplan', 'move', 'mdone'].includes(this.screen));
	domainOk = $derived(this.domainCheck?.level === 'ok');
	get target() {
		return this.input.profiles().find((p) => p.id === this.targetID);
	}

	// Установка идёт в Go сама; окно только спрашивает, как дела.
	poll: ReturnType<typeof setInterval> | undefined;
	savedRequest = 0;

	constructor(input: PanelInput) {
		// Поля с инициализаторами исполняются до тела конструктора, поэтому всё,
		// что читает input, ставится здесь — первым делом.
		this.input = input;
		this.api = untrack(() => backend(input.profile().id));
		this.sessionRemember = untrack(() => input.profile().remember);
		$effect(() => {
			const r = this.input.request();
			if (r && r.id !== this.seenRequest) {
				this.seenRequest = r.id;
				untrack(() => {
					this.password = r.password;
					this.sessionRemember = this.input.profile().remember;
					this.pendingAuto = r.auto;
					this.makeKey = r.makeKey ?? false;
					this.input.onconsumed();
					void this.attempt(() => this.connect());
				});
			}
		});
		$effect(() => {
			const value = this.input.reconnect();
			if (value && value !== this.seenReconnect) {
				this.seenReconnect = value;
				untrack(() => {
					void this.attempt(async () => {
						this.connection = 'connected';
						this.screen = 'inspect';
						await this.inspect(true);
					});
				});
			}
		});
		onMount(() => {
			void this.refreshSaved();
			if (!this.input.request() && this.input.profile().auto_connect)
				void this.attempt(() => this.connect());
		});
		onDestroy(() => this.stopPoll());
		// Отчёт окну: статус, занятость и подключённость этого профиля.
		$effect(() => {
			const state = this.onStateView();
			untrack(() => this.input.onstate(state));
		});
	}

	get profile() {
		return this.input.profile();
	}

	async attempt(fn: () => Promise<unknown>) {
		try {
			this.operationError = '';
			await fn();
		} catch (e) {
			this.operationError = String(e);
			if (this.connection === 'connecting' || (this.screen === 'inspect' && !this.inspection)) {
				this.connection = 'error';
				this.screen = 'offline';
			}
		}
	}

	autoConnect = async (on: boolean) => {
		await backend().SetAutoConnect(this.input.profile().id, on);
		await this.input.onprofileschange();
	}

	refreshSaved = async () => {
		const serial = ++this.savedRequest;
		const address = this.input.profile().host,
			login = this.input.profile().user;
		const next =
			address.trim() && login.trim() ? await this.api.HasSavedPassword(address, login) : false;
		if (serial === this.savedRequest && address === this.input.profile().host && login === this.input.profile().user)
			this.saved = next;
	}

	applyConnect = async (res: ConnectResult) => {
		this.failure = undefined;
		if (res.status === 'connected') {
			this.connection = 'connected';
			this.password = '';
			await this.refreshSaved();
			if (this.pendingAuto) {
				this.pendingAuto = false;
				try {
					await this.autoConnect(true);
				} catch (e) {
					this.operationError = String(e);
				}
			}
			this.keyNote = res.key_note ?? '';
			this.screen = 'inspect';
			await this.inspect(true);
		} else if (res.status === 'unknown_host' || res.status === 'changed_host') {
			this.changedFingerprint = res.status === 'changed_host';
			this.previousFingerprint = res.previous_fingerprint ?? '';
			this.connection = 'fingerprint';
			this.fingerprint = res.fingerprint ?? '';
			this.screen = 'fingerprint';
		} else {
			this.connection = res.status === 'password_required' ? 'password_required' : 'error';
			this.failure = res;
			this.screen = 'offline';
		}
	}

	connect = async () => {
		if (this.busy) return;
		this.busy = true;
		this.connection = 'connecting';
		this.page = undefined;
		this.inspection = undefined;
		this.plan = undefined;
		this.progress = undefined;
		this.screen = 'offline';
		try {
			const profile = this.input.profile();
			if (!profile.use_keys && this.sessionRemember !== profile.remember) {
				await this.api.SaveProfile({
					...profile,
					remember: this.sessionRemember,
					auto_connect: profile.auto_connect && this.sessionRemember
				});
				await this.input.onprofileschange();
			}
			await this.applyConnect(await this.api.Connect(this.password, this.makeKey));
		} finally {
			this.busy = false;
		}
	}

	confirmHost = async () => {
		if (this.busy) return;
		this.busy = true;
		try {
			await this.applyConnect(
				await (this.changedFingerprint ? this.api.ConfirmChangedHost() : this.api.ConfirmHost())
			);
		} finally {
			this.busy = false;
		}
	}

	inspect = async (landing = false) => {
		this.busy = true;
		this.inspection = undefined;
		this.domainCheck = undefined;
		try {
			const insp = await this.api.Inspect();
			this.inspection = insp;
			// Wynd здесь уже стоит — сразу страница сервера (владелец, 2026-10-24: 7а).
			if (insp.ok && insp.report.findings?.some((f) => f.id === 'wynd' || f.id === 'wyndelse')) {
				const p = await this.api.ServerPage();
				if (p.ok && p.state.installed && p.state.container) {
					this.page = p;
					this.screen = 'server';
				}
			}
			if (
				insp.ok &&
				landing &&
				this.screen !== 'server' &&
				!insp.report.findings?.some(
					(f) => f.level === 'block' || f.id === 'wynd' || f.id === 'wyndelse'
				)
			)
				this.screen = 'new';
			if (!insp.ok) this.connection = 'error';
			await this.input.onprofileschange();
		} finally {
			this.busy = false;
		}
	}

	refreshPage = async () => {
		const page = await this.api.ServerPage();
		this.page = page;
		if (!page.ok) {
			this.connection = 'error';
			this.screen = 'offline';
			this.failure = {
				status: 'error',
				host: this.input.profile().host,
				message: page.message,
				advice: page.advice
			};
		}
		await this.input.onprofileschange();
	}

	// «К серверу» после обновления, бэкапа или переезда: состояние сервера
	// могло поменяться (обрыв посреди обновления — кадр 7е), читаем заново.
	toServer = async () => {
		if (this.moving && !this.progress?.running) await this.api.FinishMove();
		this.screen = 'server';
		this.svcError = '';
		const p = await this.api.ServerPage();
		if (p.ok) {
			this.page = p;
			await this.input.onprofileschange();
		} else {
			this.connection = 'error';
			this.screen = 'offline';
			this.failure = {
				status: 'error',
				host: this.input.profile().host,
				message: p.message,
				advice: p.advice
			};
		}
	}

	service = async (action: 'start' | 'stop' | 'restart') => {
		if (this.svcBusy) return;
		this.svcBusy = action;
		this.svcError = '';
		try {
			const res = await this.api.Service(action);
			this.page = res.page;
			this.svcLog = res.log ?? [];
			if (!res.ok) this.svcError = [res.message, res.advice].filter(Boolean).join('. ');
			this.askStop = false;
			await this.input.onprofileschange();
		} finally {
			this.svcBusy = '';
		}
	}

	toUpdatePlan = () => {
		const page = this.page;
		if (!page) return;
		this.stopForCopy = true;
		this.startAfter = !page.state.stopped_at;
		// Собран на сервере — и новый собираем, а не молча берём из хранилища (7б4).
		this.buildUpdate = page.built && page.can_build;
		this.screen = 'uplan';
	}

	startUpdate = async () => {
		const page = this.page;
		if (!page) return;
		this.upFrom = page.state.update?.from ?? page.state.version ?? '';
		this.upTo = page.state.update?.to ?? page.latest ?? '';
		this.screen = 'update';
		this.applyUpdate(
			await this.api.StartUpdate({
				stop_for_copy: this.stopForCopy || !page.live_copy,
				start_after: page.state.running || this.startAfter,
				build: this.buildUpdate
			})
		);
		if (this.progress?.running && !this.poll) {
			this.poll = setInterval(() => void this.api.InstallProgress().then((p) => this.applyUpdate(p)), 500);
		}
	}
	applyUpdate = (next: Progress) => {
		this.progress = next;
		if (next.running) return;
		this.stopPoll();
		if (next.done) {
			this.screen = 'udone';
			void this.refreshPage();
		}
	}

	toBackupPlan = async () => {
		const page = this.page;
		if (!page) return;
		const key = 'wynd-workshop-backup:' + (page.state.domain ?? '');
		let saved = '';
		try {
			saved = localStorage.getItem(key) ?? '';
		} catch {
			saved = '';
		}
		this.bFolder = saved || (await this.api.DefaultBackupFolder(page.state.domain ?? 'wynd'));
		this.bSite = page.live_copy ? 'keep' : 'pause';
		this.bPass = this.bPass2 = '';
		this.screen = 'bplan';
		await this.planBackup();
	}
	planBackup = async () => {
		const plan = await this.api.PlanBackup(this.bFolder);
		this.bPlan = plan;
		this.bEncrypt = Boolean(plan.encrypted);
	}
	chooseBackupFolder = async () => {
		const p = await this.api.PickFolder();
		if (p) {
			this.bFolder = p;
			await this.planBackup();
		}
	}
	passOk = $derived(
		!this.bEncrypt || (this.bPass.length > 0 && (this.bPlan?.encrypted || this.bPass === this.bPass2))
	);
	startBackup = async () => {
		const page = this.page;
		if (!page || !this.bFolder || !this.passOk) return;
		try {
			localStorage.setItem('wynd-workshop-backup:' + (page.state.domain ?? ''), this.bFolder);
		} catch {
			/* без памяти папки — спросим её в следующий раз */
		}
		this.screen = 'backup';
		const apply = (next: Progress) => {
			this.progress = next;
			if (next.running) return;
			this.stopPoll();
			if (next.done) {
				this.screen = 'bdone';
				void this.refreshPage();
			}
		};
		apply(
			await this.api.StartBackup({
				folder: this.bFolder,
				site: page.state.running ? this.bSite : 'keep',
				password: this.bEncrypt ? this.bPass : ''
			})
		);
		if (this.progress?.running && !this.poll) {
			this.poll = setInterval(() => void this.api.InstallProgress().then(apply), 500);
		}
	}

	toMove = () => {
		this.mIntent = '';
		this.mFail = undefined;
		this.mPass = '';
		this.screen = 'mconnect';
	}
	resumeMove = (intent: 'continue' | 'cancel') => {
		const st = this.st;
		if (!st?.move) return;
		this.mIntent = intent;
		this.targetID = this.input.profiles().find((p) => p.id !== this.input.profile().id && p.host === st.move?.to)?.id ?? '';
		this.mFail = undefined;
		this.mPass = '';
		this.screen = 'mconnect';
	}
	applyMove = async (res: ConnectResult) => {
		this.mFail = undefined;
		if (res.status === 'connected') {
			this.mPass = '';
			this.keyNote = res.key_note ?? '';
			if (this.mIntent === 'cancel') {
				await this.cancelMove(true);
				return;
			}
			this.screen = 'mplan';
			this.mplan = undefined;
			const plan = await this.api.MovePlan();
			this.mplan = plan;
			if (this.mIntent === 'continue' && plan.ok && plan.resume) {
				this.mRemoveOld = Boolean(plan.remove_old);
				await this.startMove();
			}
		} else if (res.status === 'unknown_host' || res.status === 'changed_host') {
			this.mChangedFingerprint = res.status === 'changed_host';
			this.mPreviousFingerprint = res.previous_fingerprint ?? '';
			this.mFinger = res.fingerprint ?? '';
			this.screen = 'mfinger';
		} else {
			this.mFail = res;
			this.screen = 'mconnect';
		}
	}
	mChangedFingerprint = $state(false);
	mPreviousFingerprint = $state('');
	moveConnect = async () => {
		if (this.mBusy || !this.targetID) return;
		this.mBusy = true;
		try {
			const result = await this.api.MoveSelect(this.targetID, this.mPass);
			await this.input.onprofileschange();
			await this.applyMove(result);
		} finally {
			this.mBusy = false;
		}
	}
	moveConfirm = async () => {
		if (this.mBusy) return;
		this.mBusy = true;
		try {
			await this.applyMove(
				await (this.mChangedFingerprint ? this.api.MoveConfirmChangedHost() : this.api.MoveConfirmHost())
			);
		} finally {
			this.mBusy = false;
		}
	}
	startMove = async () => {
		this.screen = 'move';
		const apply = (next: Progress) => {
			this.progress = next;
			if (next.running) return;
			this.stopPoll();
			if (next.done) {
				this.screen = 'mdone';
				void this.attempt(async () => {
					this.completedTarget = await this.api.FinishMove();
					await this.input.onmoved(this.completedTarget);
					await this.refreshPage();
				});
			}
		};
		apply(await this.api.StartMove(this.mRemoveOld));
		if (this.progress?.running && !this.poll) {
			this.poll = setInterval(() => void this.api.InstallProgress().then(apply), 500);
		}
	}
	cancelMove = async (force = false) => {
		if (this.mBusy && !force) return;
		this.mBusy = true;
		try {
			this.stopPoll();
			const res = await this.api.CancelMove();
			this.page = res.page;
			this.svcLog = res.log ?? [];
			if (!res.ok) this.svcError = [res.message, res.advice].filter(Boolean).join('. ');
			if (!res.ok) return;
			const target = await this.api.FinishMove();
			if (target) await this.input.onmoved(target);
			await this.input.onprofileschange();
			this.progress = undefined;
			this.mIntent = '';
			this.screen = 'server';
		} finally {
			this.mBusy = false;
		}
	}
	adoptNew = async () => {
		if (this.completedTarget) {
			this.input.onselect(this.completedTarget);
			await this.input.onmoved(this.completedTarget);
		} else await this.toServer();
	}

	returnPrevious = async () => {
		if (this.returning) return;
		this.returning = true;
		this.svcError = '';
		try {
			const res = await this.api.ReturnPrevious();
			this.page = res.page;
			this.svcLog = res.log ?? [];
			if (res.ok) {
				this.progress = undefined;
				this.screen = 'server';
			} else {
				this.svcError = [res.message, res.advice].filter(Boolean).join('. ');
			}
		} finally {
			this.returning = false;
		}
	}

	checkDomain = async () => {
		if (this.checkingDomain || !this.domain.trim()) return;
		this.checkingDomain = true;
		try {
			this.domainCheck = await this.api.CheckDomain(this.domain);
			if (this.domainCheck.level === 'ok') this.domain = this.domainCheck.domain;
		} finally {
			this.checkingDomain = false;
		}
	}

	toConnect = async () => {
		this.stopPoll();
		this.plan = undefined;
		this.progress = undefined;
		await this.api.Disconnect();
		this.inspection = undefined;
		this.domainCheck = undefined;
		this.failure = undefined;
		this.connection = 'disconnected';
		this.screen = 'offline';
		await this.input.onprofileschange();
	}

	toPlan = async () => {
		if (this.planning || !this.domainOk) return;
		this.planning = true;
		this.rollbackNote = '';
		try {
			if (!this.st?.installed)
				await this.api.SetInstallRoot(this.input.profile().id, this.installRoot.trim() || '/opt/wynd');
			this.plan = await this.api.MakePlan(this.domain);
			this.progress = undefined;
			this.screen = 'plan';
		} finally {
			this.planning = false;
		}
	}

	// Собрать Wynd на сервере или скачать готовым (окна 3а–3в).
	choosing = $state(false);
	chooseBuild = async (on: boolean) => {
		if (this.choosing) return;
		this.choosing = true;
		try {
			const next = await this.api.ChooseBuild(on);
			if (next.ok) this.plan = next;
		} finally {
			this.choosing = false;
		}
	}

	stopPoll = () => {
		clearInterval(this.poll);
		this.poll = undefined;
	}
	applyProgress = (next: Progress) => {
		this.progress = next;
		if (next.running) return;
		this.stopPoll();
		if (next.done) {
			this.screen = 'done';
			void this.attempt(() => this.refreshPage());
		}
	}
	install = async () => {
		this.askRollback = false;
		this.rollbackError = '';
		this.rollbackLog = [];
		this.screen = 'install';
		this.applyProgress(await this.api.StartInstall());
		if (this.progress?.running && !this.poll) {
			this.poll = setInterval(() => void this.api.InstallProgress().then((p) => this.applyProgress(p)), 500);
		}
	}

	// Чужой веб-сервер, за которым стоит Wynd: «nginx», «Apache», «Caddy» или пусто — свой Caddy.
	proxyName = $derived(
		({ nginx: 'nginx', apache: 'Apache', caddy: 'Caddy' } as Record<string, string>)[
			this.inspection?.report?.proxy ?? ''
		] ?? ''
	);
	// Файлы сайта в чужом веб-сервере, которые ставили не мы (кадры 5е, 5е2):
	// Wynd там поставлен не Мастерской; файл не трогаем, а называем.
	foreignSite = $derived(this.proxyName ? (this.inspection?.report?.foreign_site ?? []) : []);

	rollback = async () => {
		if (this.rolling) return;
		this.rolling = true;
		this.rollbackError = '';
		try {
			const res = await this.api.Rollback(!this.dropData);
			if (res.ok) {
				const failed = this.screen === 'install';
				this.askRollback = false;
				this.rollbackLog = [];
				this.progress = undefined;
				if (res.report) this.inspection = { ok: true, report: res.report };
				if (res.plan) this.plan = res.plan;
				// Осмотр после отката тот же веб-сервер найдёт снова; текст итога — lib/rollback.
				this.rollbackNote = removalNote({
					dropData: this.dropData,
					failed,
					proxyName: this.proxyName,
					foreignSite: this.foreignSite,
					cert: res.cert,
					stale: res.stale
				});
				this.screen = failed ? 'plan' : 'new';
				this.page = await this.api.ServerPage();
				await this.input.onprofileschange();
			} else {
				this.rollbackError = [res.message, res.advice].filter(Boolean).join('. ');
				// Что откат успел и на чём встал — в тех же «подробностях».
				this.rollbackLog = res.log ?? [];
			}
		} finally {
			this.rolling = false;
		}
	}

	copyRaw = async () => {
		this.copiedRaw = await this.api.CopyText(this.inspection?.report.raw ?? '');
		setTimeout(() => (this.copiedRaw = false), 2000);
	}

	copyLine = async (line: string) => {
		if (!(await this.api.CopyText(line))) return;
		this.copiedLine = line;
		setTimeout(() => (this.copiedLine = ''), 2000);
	}

	copyLink = async () => {
		if (!this.progress?.link) return;
		this.copied = await this.api.CopyText(this.progress.link);
		setTimeout(() => (this.copied = false), 2000);
	}

	occupied = $derived(
		this.busy ||
			Boolean(this.svcBusy) ||
			this.rolling ||
			this.returning ||
			this.mBusy ||
			this.planning ||
			this.choosing ||
			this.checkingDomain ||
			Boolean(this.progress?.running)
	);
	get status() {
		return this.input.profile().reserved_by
			? 'Назначение переезда'
			: this.progress?.running
				? `${this.moving ? 'Переезд' : this.updating ? 'Обновление' : this.backingUp ? 'Бэкап' : 'Установка'} · ${this.progress.steps?.find((s) => s.status === 'running')?.title ?? 'выполняется'}`
				: this.connection === 'connected'
					? 'Подключён'
					: this.connection === 'connecting'
						? 'Подключаемся…'
						: this.connection === 'fingerprint'
							? 'Проверьте отпечаток'
							: this.connection === 'password_required'
								? 'Нужен пароль'
								: this.connection === 'error'
									? 'Нет подключения'
									: 'Нет подключения';
	}

	logText = $derived(
		[...(this.progress?.log ?? []), ...this.rollbackLog, ...this.svcLog]
			.map((e) => ('$ ' + e.command + '\n' + e.output).trimEnd())
			.join('\n\n')
	);
	// C71: в раскрытии — журнал сборки текущей операции, а не все команды.
	buildLogText = $derived(
		(this.progress?.log ?? [])
			.filter((e) => e.command.includes('docker build'))
			.map((e) => ('$ ' + e.command + '\n' + e.output).trimEnd())
			.join('\n\n')
	);
	// Полный вывод сборки живёт на сервере (C71): окно держит показанное, ранние
	// порции догружает по кнопке. Пометка «начало не сохранено» — служебная
	// строка Go в хвосте; после догрузки она не показывается.
	earlierBuildLog = $state('');
	buildLogStart = $state(-1);
	buildLogTotal = $state(-1);
	logLoading = $state(false);
	logError = $state('');
	static readonly logTruncatedMarker = '… (начало вывода не сохранено';
	logTruncated = $derived(this.buildLogText.includes(Panel.logTruncatedMarker));
	logStartShown = $derived(this.buildLogStart === 0);
	// Хвост без служебной строки Go: на сервере её нет.
	tailWithoutMarker = $derived(
		this.buildLogText
			.split('\n')
			.filter((line) => !line.startsWith(Panel.logTruncatedMarker))
			.join('\n')
	);
	shownBuildLog = $derived(
		this.earlierBuildLog
			? (this.earlierBuildLog + this.tailWithoutMarker).trimEnd()
			: this.buildLogText
	);

		// loadEarlierBuildLog — догрузить порцию журнала до показанной (C71).
		// Порция — ровно недостающее до показанного [offset, start): последний
		// шаг берёт остаток от нуля, а не полные 256 КБ, иначе хвост начала
		// дублируется (живой прогон C71 на wynd2, 2026-11-22: перекрытие
		// 198 663 байт при не выровненном на 256 КБ размере журнала).
		async loadEarlierBuildLog(): Promise<void> {
			if (this.logLoading || this.logStartShown) return;
			this.logLoading = true;
			this.logError = '';
			try {
				const want = nextLogChunk(this.buildLogStart);
				const chunk = await this.api.BuildLogChunk(want.offset, want.limit);
			if (chunk.error) throw new Error(chunk.error);
			this.buildLogTotal = chunk.total;
			if (this.buildLogStart < 0) {
				// Первый запрос — только размер: хвост уже показан окном.
				const tailBytes = new TextEncoder().encode(this.tailWithoutMarker).length;
				this.buildLogStart = Math.max(0, chunk.total - tailBytes);
				return;
			}
			this.earlierBuildLog = chunk.text + this.earlierBuildLog;
			this.buildLogStart = chunk.offset;
		} catch {
			this.logError = 'Не удалось загрузить начало журнала. Попробуйте ещё раз.';
		} finally {
			this.logLoading = false;
		}
	}

	onStateView(): ProfileViewState {
		return {
			status: this.status,
			busy: this.occupied || Boolean(this.input.profile().reserved_by),
			connected: this.connection === 'connected'
		};
	}
}
