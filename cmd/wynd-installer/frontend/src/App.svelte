<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
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
	import StepList from '$ui/workshop/StepList.svelte';
	import {
		backend,
		type ServerProfile,
		type ProfileRequest,
		type ProfileViewState,
		type ConnectResult,
		type DomainCheck,
		type Finding,
		type InspectResult,
		type LogEntry,
		type LocalBackupPlan,
		type MovePlan,
		type Plan,
		type Progress,
		type ServerPage
	} from './api';
	import { rawSections } from './raw';

	let {
		profile,
		profiles,
		request,
		reconnect = 0,
		onconsumed,
		onstate,
		onprofileschange,
		onedit,
		onforget,
		onaddtarget,
		onselect,
		onmoved
	}: {
		profile: ServerProfile;
		profiles: ServerProfile[];
		request?: ProfileRequest;
		reconnect?: number;
		onconsumed: () => void;
		onstate: (s: ProfileViewState) => void;
		onprofileschange: () => Promise<void>;
		onedit: () => void;
		onforget: () => void;
		onaddtarget: () => void;
		onselect: (id: string) => void;
		onmoved: (id: string) => Promise<void>;
	} = $props();
	let operationError = $state('');
	const api = untrack(() => backend(profile.id));
	let connection = $state('disconnected');
	let pendingAuto = false;
	let targetID = $state('');
	let completedTarget = $state('');
	const target = $derived(profiles.find((p) => p.id === targetID));
	let seenRequest = 0;
	$effect(() => {
		const r = request;
		if (r && r.id !== seenRequest) {
			seenRequest = r.id;
			untrack(() => {
				password = r.password;
				sessionRemember = profile.remember;
				pendingAuto = r.auto;
				makeKey = r.makeKey ?? false;
				onconsumed();
				void attempt(connect);
			});
		}
	});
	let seenReconnect = 0;
	$effect(() => {
		const value = reconnect;
		if (value && value !== seenReconnect) {
			seenReconnect = value;
			untrack(() => {
				void attempt(async () => {
					connection = 'connected';
					screen = 'inspect';
					await inspect(true);
				});
			});
		}
	});
	onMount(() => {
		void refreshSaved();
		if (!request && profile.auto_connect) void attempt(connect);
	});
	onDestroy(() => stopPoll());
	async function attempt(fn: () => Promise<unknown>) {
		try {
			operationError = '';
			await fn();
		} catch (e) {
			operationError = String(e);
			if (connection === 'connecting' || (screen === 'inspect' && !inspection)) {
				connection = 'error';
				screen = 'offline';
			}
		}
	}
	async function autoConnect(on: boolean) {
		await backend().SetAutoConnect(profile.id, on);
		await onprofileschange();
	}
	// Окна установщика (docs/visual/installer.html): 1 — подключение, 1а —
	// отпечаток, 2 — осмотр, 2а — помеха, 3 — план, 4 — установка, 4а — шаг не
	// прошёл, 5 — готово. Пульт сервера (6) — следующий срез.
	type Screen =
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

	let screen = $state<Screen>('offline');
	const host = $derived(profile.host);
	const user = $derived(profile.user);
	let password = $state('');
	let sessionRemember = $state(untrack(() => profile.remember));
	const useKeys = $derived(profile.use_keys);
	let makeKey = $state(false);
	let keyNote = $state('');
	let saved = $state(false);
	let busy = $state(false);
	let failure = $state<ConnectResult | undefined>();
	let fingerprint = $state('');
	let previousFingerprint = $state('');
	let changedFingerprint = $state(false);
	let mPreviousFingerprint = $state('');
	let mChangedFingerprint = $state(false);

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

	const updating = $derived(['uplan', 'update', 'udone'].includes(screen));
	const backingUp = $derived(['bplan', 'backup', 'bdone'].includes(screen));
	const moving = $derived(['mconnect', 'mfinger', 'mplan', 'move', 'mdone'].includes(screen));
	const domainOk = $derived(domainCheck?.level === 'ok');

	// Есть сохранённый пароль — поле можно оставить пустым.
	let savedRequest = 0;
	async function refreshSaved() {
		const serial = ++savedRequest;
		const address = host,
			login = user;
		const next =
			address.trim() && login.trim() ? await api.HasSavedPassword(address, login) : false;
		if (serial === savedRequest && address === host && login === user) saved = next;
	}

	async function applyConnect(res: ConnectResult) {
		failure = undefined;
		if (res.status === 'connected') {
			connection = 'connected';
			password = '';
			await refreshSaved();
			if (pendingAuto) {
				pendingAuto = false;
				try {
					await autoConnect(true);
				} catch (e) {
					operationError = String(e);
				}
			}
			keyNote = res.key_note ?? '';
			screen = 'inspect';
			await inspect(true);
		} else if (res.status === 'unknown_host' || res.status === 'changed_host') {
			changedFingerprint = res.status === 'changed_host';
			previousFingerprint = res.previous_fingerprint ?? '';
			connection = 'fingerprint';
			fingerprint = res.fingerprint ?? '';
			screen = 'fingerprint';
		} else {
			connection = res.status === 'password_required' ? 'password_required' : 'error';
			failure = res;
			screen = 'offline';
		}
	}

	async function connect() {
		if (busy) return;
		busy = true;
		connection = 'connecting';
		page = undefined;
		inspection = undefined;
		plan = undefined;
		progress = undefined;
		screen = 'offline';
		try {
			if (!profile.use_keys && sessionRemember !== profile.remember) {
				await api.SaveProfile({
					...profile,
					remember: sessionRemember,
					auto_connect: profile.auto_connect && sessionRemember
				});
				await onprofileschange();
			}
			await applyConnect(await api.Connect(password, makeKey));
		} finally {
			busy = false;
		}
	}

	async function confirmHost() {
		if (busy) return;
		busy = true;
		try {
			await applyConnect(await (changedFingerprint ? api.ConfirmChangedHost() : api.ConfirmHost()));
		} finally {
			busy = false;
		}
	}

	async function inspect(landing = false) {
		busy = true;
		inspection = undefined;
		domainCheck = undefined;
		try {
			inspection = await api.Inspect();
			// Wynd здесь уже стоит — сразу страница сервера (владелец, 2026-10-24: 7а).
			if (
				inspection.ok &&
				inspection.report.findings?.some((f) => f.id === 'wynd' || f.id === 'wyndelse')
			) {
				const p = await api.ServerPage();
				if (p.ok && p.state.installed && p.state.container) {
					page = p;
					screen = 'server';
				}
			}
			if (
				inspection.ok &&
				landing &&
				screen !== 'server' &&
				!inspection.report.findings?.some(
					(f) => f.level === 'block' || f.id === 'wynd' || f.id === 'wyndelse'
				)
			)
				screen = 'new';
			if (!inspection.ok) connection = 'error';
			await onprofileschange();
		} finally {
			busy = false;
		}
	}

	// Пульт (кадры 7а–7ж2).
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
		return Boolean(
			p && p.state.free_bytes > 0 && p.state.free_bytes < p.copy_bytes + 64 * 1024 * 1024
		);
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
		page = await api.ServerPage();
		if (!page.ok) {
			connection = 'error';
			screen = 'offline';
			failure = { status: 'error', host: profile.host, message: page.message, advice: page.advice };
		}
		await onprofileschange();
	}

	// «К серверу» после обновления, бэкапа или переезда: состояние сервера
	// могло поменяться (обрыв посреди обновления — кадр 7е), читаем заново.
	async function toServer() {
		if (moving && !progress?.running) await api.FinishMove();
		screen = 'server';
		svcError = '';
		const p = await api.ServerPage();
		if (p.ok) {
			page = p;
			await onprofileschange();
		} else {
			connection = 'error';
			screen = 'offline';
			failure = { status: 'error', host: profile.host, message: p.message, advice: p.advice };
		}
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
			const res = await api.Service(action);
			page = res.page;
			svcLog = res.log ?? [];
			if (!res.ok) svcError = [res.message, res.advice].filter(Boolean).join('. ');
			askStop = false;
			await onprofileschange();
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
			await api.StartUpdate({
				stop_for_copy: stopForCopy || !page.live_copy,
				start_after: page.state.running || startAfter,
				build: buildUpdate
			})
		);
		if (progress?.running && !poll) {
			poll = setInterval(() => void api.InstallProgress().then(applyUpdate), 500);
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
		bFolder = saved || (await api.DefaultBackupFolder(page.state.domain ?? 'wynd'));
		bSite = page.live_copy ? 'keep' : 'pause';
		bPass = bPass2 = '';
		screen = 'bplan';
		await planBackup();
	}
	async function planBackup() {
		bPlan = await api.PlanBackup(bFolder);
		bEncrypt = Boolean(bPlan.encrypted);
	}
	async function chooseBackupFolder() {
		const p = await api.PickFolder();
		if (p) {
			bFolder = p;
			await planBackup();
		}
	}
	const passOk = $derived(
		!bEncrypt || (bPass.length > 0 && (bPlan?.encrypted || bPass === bPass2))
	);
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
			await api.StartBackup({
				folder: bFolder,
				site: page.state.running ? bSite : 'keep',
				password: bEncrypt ? bPass : ''
			})
		);
		if (progress?.running && !poll) {
			poll = setInterval(() => void api.InstallProgress().then(apply), 500);
		}
	}

	// Переезд (9а–9г).
	let mPass = $state('');
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
		targetID = profiles.find((p) => p.id !== profile.id && p.host === st.move?.to)?.id ?? '';
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
			mplan = await api.MovePlan();
			if (mIntent === 'continue' && mplan.ok && mplan.resume) {
				mRemoveOld = Boolean(mplan.remove_old);
				await startMove();
			}
		} else if (res.status === 'unknown_host' || res.status === 'changed_host') {
			mChangedFingerprint = res.status === 'changed_host';
			mPreviousFingerprint = res.previous_fingerprint ?? '';
			mFinger = res.fingerprint ?? '';
			screen = 'mfinger';
		} else {
			mFail = res;
			screen = 'mconnect';
		}
	}
	async function moveConnect() {
		if (mBusy || !targetID) return;
		mBusy = true;
		try {
			const result = await api.MoveSelect(targetID, mPass);
			await onprofileschange();
			await applyMove(result);
		} finally {
			mBusy = false;
		}
	}
	async function moveConfirm() {
		if (mBusy) return;
		mBusy = true;
		try {
			await applyMove(
				await (mChangedFingerprint ? api.MoveConfirmChangedHost() : api.MoveConfirmHost())
			);
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
			if (next.done) {
				screen = 'mdone';
				void attempt(async () => {
					completedTarget = await api.FinishMove();
					await onmoved(completedTarget);
					await refreshPage();
				});
			}
		};
		apply(await api.StartMove(mRemoveOld));
		if (progress?.running && !poll) {
			poll = setInterval(() => void api.InstallProgress().then(apply), 500);
		}
	}
	async function cancelMove(force = false) {
		if (mBusy && !force) return;
		mBusy = true;
		try {
			stopPoll();
			const res = await api.CancelMove();
			page = res.page;
			svcLog = res.log ?? [];
			if (!res.ok) svcError = [res.message, res.advice].filter(Boolean).join('. ');
			if (!res.ok) return;
			const target = await api.FinishMove();
			if (target) await onmoved(target);
			await onprofileschange();
			progress = undefined;
			mIntent = '';
			screen = 'server';
		} finally {
			mBusy = false;
		}
	}
	async function adoptNew() {
		if (completedTarget) {
			onselect(completedTarget);
			await onmoved(completedTarget);
		} else await toServer();
	}

	async function returnPrevious() {
		if (returning) return;
		returning = true;
		svcError = '';
		try {
			const res = await api.ReturnPrevious();
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
			domainCheck = await api.CheckDomain(domain);
			if (domainCheck.level === 'ok') domain = domainCheck.domain;
		} finally {
			checkingDomain = false;
		}
	}

	async function toConnect() {
		stopPoll();
		plan = undefined;
		progress = undefined;
		await api.Disconnect();
		inspection = undefined;
		domainCheck = undefined;
		failure = undefined;
		connection = 'disconnected';
		screen = 'offline';
		await onprofileschange();
	}

	function mark(level: Finding['level']): string {
		return level === 'ok' ? '✓' : '!';
	}

	async function toPlan() {
		if (planning || !domainOk) return;
		planning = true;
		rollbackNote = '';
		try {
			plan = await api.MakePlan(domain);
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
			const next = await api.ChooseBuild(on);
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
		if (next.done) {
			screen = 'done';
			void attempt(refreshPage);
		}
	}
	async function install() {
		askRollback = false;
		rollbackError = '';
		rollbackLog = [];
		screen = 'install';
		applyProgress(await api.StartInstall());
		if (progress?.running && !poll) {
			poll = setInterval(() => void api.InstallProgress().then(applyProgress), 500);
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
		({ nginx: 'nginx', apache: 'Apache', caddy: 'Caddy' } as Record<string, string>)[
			inspection?.report?.proxy ?? ''
		] ?? ''
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
			const res = await api.Rollback(!dropData);
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
					screen = 'new';
				}
				page = await api.ServerPage();
				await onprofileschange();
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
		copiedRaw = await api.CopyText(inspection?.report.raw ?? '');
		setTimeout(() => (copiedRaw = false), 2000);
	}

	async function copyLine(line: string) {
		if (!(await api.CopyText(line))) return;
		copiedLine = line;
		setTimeout(() => (copiedLine = ''), 2000);
	}

	async function copyLink() {
		if (!progress?.link) return;
		copied = await api.CopyText(progress.link);
		setTimeout(() => (copied = false), 2000);
	}

	const occupied = $derived(
		busy ||
			Boolean(svcBusy) ||
			rolling ||
			returning ||
			mBusy ||
			planning ||
			choosing ||
			checkingDomain ||
			Boolean(progress?.running)
	);
	const status = $derived(
		profile.reserved_by
			? 'Назначение переезда'
			: progress?.running
				? `${moving ? 'Переезд' : updating ? 'Обновление' : backingUp ? 'Бэкап' : 'Установка'} · ${progress.steps?.find((s) => s.status === 'running')?.title ?? 'выполняется'}`
				: connection === 'connected'
					? 'Подключён'
					: connection === 'connecting'
						? 'Подключаемся…'
						: connection === 'fingerprint'
							? 'Проверьте отпечаток'
							: connection === 'password_required'
								? 'Нужен пароль'
								: connection === 'error'
									? 'Нет подключения'
									: 'Нет подключения'
	);
	$effect(() => {
		const state = {
			status,
			busy: occupied || Boolean(profile.reserved_by),
			connected: connection === 'connected'
		};
		untrack(() => onstate(state));
	});

	const logText = $derived(
		[...(progress?.log ?? []), ...rollbackLog, ...svcLog]
			.map((e) => ('$ ' + e.command + '\n' + e.output).trimEnd())
			.join('\n\n')
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
				Файл сайта в {proxyName} ставили не мы, его не тронем. Пока он есть, {proxyName}
				отвечает по адресу
				{site || 'сайта'} ошибкой. Уберите его сами:
			</div>
			{#each foreignSite as f (f)}
				<CopyLine text={f} copied={copiedLine === f} oncopy={() => void copyLine(f)} />
			{/each}
		{/if}
		<Checkbox bind:checked={dropData}>
			Удалить данные Wynd — записи, фотографии, учётные записи{foreignSite.length
				? ''
				: ' — и сертификат сайта'}
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
			<Button
				variant="colored"
				loading={rolling}
				loadingLabel="Убираем Wynd…"
				onclick={() => void attempt(() => rollback())}>{yes}</Button
			>
			<Button variant="ghost" disabled={rolling} onclick={() => (askRollback = false)}
				>Отмена</Button
			>
		</FootBar>
	</Notice>
{/snippet}

<main class="ins-body ins-pane">
	<Notice
		profile={{
			name: profile.name,
			host: profile.host,
			user: profile.user,
			domain: page?.state.domain ?? profile.last_check?.domain,
			status: connection === 'connected' ? 'Подключён' : status,
			state: connection
		}}
	>
		<Button
			variant="ghost"
			loading={connection === 'connecting'}
			loadingLabel="Подключаемся…"
			disabled={occupied || moving || Boolean(profile.reserved_by)}
			onclick={() => void attempt(connection === 'connected' ? toConnect : connect)}
			>{connection === 'connected' ? 'Отключиться' : 'Подключиться'}</Button
		>
		<Checkbox
			native
			checked={profile.auto_connect}
			disabled={!profile.auto_connect && !profile.use_keys && (!saved || !profile.remember)}
			onchange={(on) => void attempt(() => autoConnect(on))}>Подключаться при запуске</Checkbox
		>
	</Notice>
	{#if updating || backingUp || moving || ['inspect', 'plan', 'install', 'done'].includes(screen)}
		<div class="ins-operation-path">{updating ? 'Обновление' : backingUp ? 'Бэкап' : moving ? 'Переезд' : 'Установка Wynd'}</div>
	{/if}
	{#if operationError}<Notice tone="bad" alert>{operationError}</Notice>{/if}
	{#if moving && target}<div class="ins-move-pair">
			<div>
				<strong>Откуда: {profile.name}</strong><small>SSH: {profile.user}@{profile.host}</small>
			</div>
			<span>→</span>
			<div>
				<strong>Куда: {target.name}</strong><small>SSH: {target.user}@{target.host}</small>
			</div>
		</div>{/if}

	{#if connection === 'connecting'}<h2>Проверяем сервер</h2>
		<p class="ins-sub">После подключения узнаем состояние Wynd. Можно выбрать другой сервер.</p>
	{:else if profile.reserved_by}<h2>Сервер участвует в переезде</h2>
		<p class="ins-sub">
			Выбран назначением для сервера {profiles.find((p) => p.id === profile.reserved_by)?.name}.
			Управление переездом — в пульте исходного сервера.
		</p>
	{:else if screen === 'offline' || screen === 'connect'}
		<h2>
			{connection === 'password_required'
				? 'Для входа нужен пароль'
				: failure
					? 'Не удалось подключиться'
					: 'Подключитесь, чтобы проверить сервер'}
		</h2>
		{#if failure}<Notice tone="bad" alert title={failure.message} text={failure.advice}
				>{#if failure.fingerprint}<div class="ins-mono">
						{failure.fingerprint}
					</div>{/if}</Notice
			>{/if}
		{#if !profile.use_keys}<Label>Пароль от сервера</Label><Input
				type="password"
				autocomplete="off"
				bind:value={password}
				placeholder={saved ? 'сохранён — можно не вводить' : ''}
			/><Checkbox native bind:checked={sessionRemember}
				>Запомнить пароль на этом компьютере</Checkbox
			>
		{/if}
		{#if profile.last_check}{@const c = profile.last_check}<Notice
				title={`Последняя проверка: ${dayTime(c.at)}`}
				><p>
					{c.installed
						? `Wynd ${c.version ?? ''} ${c.running ? 'работал' : 'не работал'}`
						: 'Wynd не был установлен'}{c.free_bytes ? ` · свободно ${gb(c.free_bytes)}` : ''}.
				</p>
				<p class="ins-dim">
					Это сохранённые сведения. Текущее состояние узнаем после подключения.
				</p></Notice
			>{/if}
		<FootBar
			><Button
				variant="colored"
				loading={busy}
				loadingLabel="Подключаемся…"
				disabled={Boolean(profile.reserved_by)}
				onclick={() => void attempt(connect)}>{failure ? 'Повторить' : 'Подключиться'}</Button
			><Button
				variant="ghost"
				class="ins-push"
				disabled={occupied || moving || Boolean(profile.reserved_by)}
				onclick={onedit}>Изменить доступ</Button
			></FootBar
		>
	{:else if screen === 'new'}
		{#if rollbackNote}<Notice tone="note">{rollbackNote}</Notice>{/if}
		<h2>Wynd на сервере не найден</h2>
		<p class="ins-sub">Проверено сейчас.</p>
		<Notice
			>Можно установить Wynd. Сначала Мастерская проверит, что потребуется изменить, и покажет план.</Notice
		>
		<FootBar
			><Button variant="colored" onclick={() => (screen = 'inspect')}>Установить Wynd</Button
			></FootBar
		>
	{:else if screen === 'server' && page && st}
		<!-- Пульт (кадры 7а, 7а2, 7а3, 7е). -->
		{#if keyNote}<Notice tone="note">{keyNote}</Notice>{/if}
		<h2>
			{page.foreign
				? 'Этот Wynd ставила не Мастерская'
				: st.running
					? 'Wynd работает'
					: st.stopped_at
						? 'Wynd остановлен'
						: 'Wynd не работает'}
		</h2>
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
							: `программа ${f.name}`}, данные — в {f.data_dir}. Обновлять такой Wynd, делать его
					бэкап и убирать Мастерская не умеет. Перевезти на другой сервер — умеет: там она поставит
					Wynd сама и перенесёт данные и адрес сайта.
				</div>
			</Notice>
			<MarkRows class="mt-10">
				<MarkRow mark="→">
					Переехать на другой сервер
					{#snippet action()}
						<TextButton class="link under" onclick={toMove}>Перенести</TextButton>
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
					<Button variant="colored" onclick={() => resumeMove('continue')}
						>Продолжить переезд</Button
					>
					<Button variant="ghost" onclick={() => resumeMove('cancel')}>Отменить переезд</Button>
				</FootBar>
			</Notice>
		{:else if st.update}
			<Notice title="Обновление до {st.update.to} не закончено">
				<div class="ins-dim">
					<!-- Момент обрыва Мастерская не знает — время начала обновления (владелец, 2026-10-25). -->
					Обновление началось {dayTime(st.update.started)}. Связь с сервером прервалась{st.update
						.stopped
						? ', когда Wynd был остановлен,'
						: ''}{st.running ? '' : ' — с тех пор сайт не открывается'}. {st.update.copied
						? 'Копия данных перед обновлением на месте.'
						: 'Копию сделать не успели — данные не менялись.'}
				</div>
				<FootBar>
					<Button variant="colored" onclick={() => void attempt(() => startUpdate())}
						>Продолжить обновление</Button
					>
					<Button
						variant="ghost"
						loading={returning}
						loadingLabel="Возвращаем…"
						onclick={() => void attempt(() => returnPrevious())}
					>
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
						loadingLabel="Перезапускаем…"
						disabled={Boolean(svcBusy)}
						onclick={() => void attempt(() => service('restart'))}>Перезапустить</Button
					>
					<Button variant="ghost" disabled={Boolean(svcBusy)} onclick={() => (askStop = true)}>
						Остановить
					</Button>
				{:else}
					<Button
						variant="colored"
						loading={svcBusy === 'start'}
						loadingLabel="Запускаем…"
						onclick={() => void attempt(() => service('start'))}
					>
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
						<Button
							variant="colored"
							loading={svcBusy === 'stop'}
							loadingLabel="Останавливаем…"
							onclick={() => void attempt(() => service('stop'))}
						>
							Остановить
						</Button>
						<Button variant="ghost" onclick={() => (askStop = false)}>Отмена</Button>
					</FootBar>
				</Notice>
			{/if}
			{#if page.latest}
				<Notice class="ins-pult-update" title="Вышла версия {page.latest}">
					<div class="ins-dim">
						Перед обновлением сохраним копию данных на сервере.
					</div>
					<FootBar>
						<Button variant={st.running ? 'colored' : 'ghost'} onclick={toUpdatePlan}
							>Обновить</Button
						>
					</FootBar>
				</Notice>
			{:else}
				<p class="ins-dim">Стоит самая свежая версия.</p>
			{/if}
			<MarkRows class="ins-pult-actions">
				<MarkRow mark="">
					Бэкап на этот компьютер
					{#snippet action()}
						<TextButton class="link under" onclick={() => void attempt(() => toBackupPlan())}
							>Сделать</TextButton
						>
					{/snippet}
				</MarkRow>
				<MarkRow mark="">
					Переезд на другой сервер
					{#snippet action()}
						<TextButton class="link under" onclick={toMove}>Перенести</TextButton>
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
		{@const get = buildUpdate
			? `Соберём Wynd ${page.latest} из исходного кода`
			: `Скачаем Wynd ${page.latest}`}
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
				{st.running || startAfter
					? `Запустим ${page.latest} и проверим сайт`
					: `Поставим ${page.latest}`}
			</MarkRow>
		</MarkRows>
		{#if st.running}
			<Checkbox bind:checked={stopForCopy} disabled={!page.live_copy}>
				Остановить сайт на время копии
				<span class="ins-dim">
					{#if !page.live_copy}
						— Wynd {st.version} копию без остановки не умеет.
					{/if}
					Пока идёт копия, никто ничего не напишет — при возврате к {st.version} ничего не пропадёт. Без
					галочки сайт закрыт только на запуск новой версии, но то, что напишут во время копии, при возврате
					пропадёт.
				</span>
			</Checkbox>
		{:else}
			<Label>После обновления</Label>
			<Radio name="after" value={true} bind:group={startAfter}>Запустить новую версию</Radio>
			<Radio name="after" value={false} bind:group={startAfter}>
				Оставить сайт остановленным
				{#if st.stopped_at}<span class="ins-dim">— его остановили вы {day(st.stopped_at)}</span
					>{/if}
			</Radio>
		{/if}
		<p class="ins-sub mt">
			Не тронем: Docker, {proxyName || 'Caddy'}, сертификат сайта.
		</p>
		{#if page.can_build}
			<p class="ins-sub">
				{buildUpdate ? 'Сервер соберёт Wynd сам.' : 'Сервер скачает Wynd готовым.'}
				<TextButton class="link under" onclick={() => (buildUpdate = !buildUpdate)}>
					{buildUpdate ? 'Скачать готовый' : 'Собрать самому'}
				</TextButton>
			</p>
		{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void attempt(() => startUpdate())}>Обновить</Button>
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
				<Button
					variant="colored"
					loading={returning}
					loadingLabel="Возвращаем…"
					onclick={() => void attempt(() => returnPrevious())}
				>
					Вернуть {upFrom}
				</Button>
				<Button
					variant="ghost"
					disabled={returning}
					onclick={() => void attempt(() => startUpdate())}>Повторить</Button
				>
			{:else}
				<Button variant="colored" onclick={() => void attempt(() => startUpdate())}
					>Повторить</Button
				>
				<Button variant="ghost" onclick={() => void attempt(() => toServer())}>К серверу</Button>
			{/if}
			<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}
				>подробности</TextButton
			>
		</FootBar>
		{#if showLog}
			<CommandLog text={logText} />
		{/if}
	{:else if screen === 'update'}
		<h2>Обновляем</h2>
		<StepList meter class="ins-stage-steps" steps={progress?.steps ?? []} />
		<FootBar>
			<TextButton class="link under" onclick={() => (showLog = !showLog)}>
				показать, что выполняется
			</TextButton>
			<span class="ins-dim ins-push">Окно можно свернуть</span>
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
				<Button
					variant="colored"
					onclick={() => void api.OpenLink(`https://${page?.state.domain}`)}
				>
					Открыть {page?.state.domain}
				</Button>
			{/if}
			<Button variant="ghost" onclick={() => void attempt(() => toServer())}>К серверу</Button>
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
			<Input
				mono
				class="ins-grow"
				bind:value={bFolder}
				onblur={() => void attempt(() => planBackup())}
			/>
			<Button variant="ghost" onclick={() => void attempt(() => chooseBackupFolder())}
				>Выбрать</Button
			>
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
				Работает, пока идёт бэкап{#if !page.live_copy}&nbsp;<span class="ins-dim"
						>— Wynd {st.version} так не умеет</span
					>{/if}
			</Radio>
			<Radio name="bsite" value="pause" bind:group={bSite}
				>Остановить на время бэкапа и запустить снова</Radio
			>
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
				variant={bFolder &&
				passOk &&
				bPlan?.ok &&
				(bSite !== 'keep' || page.live_copy || !st.running)
					? 'colored'
					: 'off'}
				onclick={() => void attempt(() => startBackup())}>Сделать бэкап</Button
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
			<Button variant="colored" onclick={() => void attempt(() => startBackup())}>Повторить</Button>
			<Button variant="ghost" onclick={() => void attempt(() => toServer())}>К серверу</Button>
			<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}
				>подробности</TextButton
			>
		</FootBar>
		{#if showLog}
			<CommandLog text={logText} />
		{/if}
	{:else if screen === 'backup'}
		<!-- Кадр 8б. -->
		<h2>Делаем бэкап</h2>
		<StepList meter class="ins-stage-steps" steps={progress?.steps ?? []} />
		<FootBar>
			<TextButton class="link under" onclick={() => (showLog = !showLog)}>
				показать, что выполняется
			</TextButton>
			<span class="ins-dim ins-push">Окно можно свернуть</span>
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
			<Button
				variant="colored"
				onclick={() => void api.ShowInFolder(bFolder + '\\wynd-backup.json')}
			>
				Открыть папку
			</Button>
			<Button variant="ghost" onclick={() => void attempt(() => toServer())}>К серверу</Button>
		</FootBar>
	{:else if screen === 'mconnect' && st}
		<h2>На какой сервер переезжаем</h2>
		<p class="ins-sub">
			Выберите сохранённый сервер или добавьте новый. Перед переездом проверим его заново.
		</p>
		{#each profiles.filter((p) => p.id !== profile.id) as dest (dest.id)}<Radio
				name={`destination-${profile.id}`}
				value={dest.id}
				bind:group={targetID}
				disabled={dest.host === profile.host ||
					Boolean(dest.busy && dest.reserved_by !== profile.id)}
				>{dest.name}<span class="ins-dim">
					— SSH: {dest.user}@{dest.host}{dest.busy && dest.reserved_by !== profile.id
						? ' · занят'
						: ''}</span
				></Radio
			>{/each}
		<TextButton class="link under" onclick={onaddtarget}>Добавить сервер</TextButton>
		{#if target && !target.use_keys}<Label>Пароль от нового сервера</Label><Input
				type="password"
				autocomplete="off"
				bind:value={mPass}
			/>{/if}
		{#if mFail}<Notice tone="bad" alert title={mFail.message} text={mFail.advice} />{/if}
		<FootBar
			><Button
				variant="colored"
				loading={mBusy}
				loadingLabel="Подключаемся…"
				disabled={!targetID}
				onclick={() => void attempt(moveConnect)}>Подключиться и проверить</Button
			><Button
				variant="ghost"
				class="ins-push"
				disabled={mBusy}
				onclick={() => void attempt(toServer)}>Отмена</Button
			></FootBar
		>
	{:else if screen === 'mfinger'}
		<h2>
			{mChangedFingerprint ? 'У нового сервера другой отпечаток' : 'Это ваш новый сервер?'}
		</h2>
		<p class="ins-sub">
			{#if mChangedFingerprint}Подключение остановлено. Сверьте новый отпечаток в панели хостинга.
				Если вы не переустанавливали сервер, не доверяйте новому ключу.{:else}Подключаемся к {target?.host ??
					''} впервые. Сервер назвал свой отпечаток — сверьте его с письмом хостинга или панелью, если
				там он есть.{/if}
		</p>
		{#if mChangedFingerprint}<Label>Сохранённый отпечаток</Label><Notice mono
				>{mPreviousFingerprint}</Notice
			><Label>Новый отпечаток</Label>{/if}<Notice mono>{mFinger}</Notice>
		<FootBar>
			<Button
				variant="colored"
				loading={mBusy}
				loadingLabel="Подключаемся…"
				onclick={() => void attempt(() => moveConfirm())}
				>{mChangedFingerprint ? 'Проверил, доверять новому' : 'Да, подключиться'}</Button
			>
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
			<Button variant="ghost" onclick={() => void attempt(() => toServer())}>К серверу</Button>
		</FootBar>
	{:else if screen === 'mplan' && mplan}
		<!-- Кадры 9б и 9б2 (на старом Wynd не со своим Caddy — переезжают одни данные). -->
		{@const dataOnly = mplan.data_only}
		{@const oldProxy =
			({ nginx: 'nginx', apache: 'Apache', caddy: 'Caddy' } as Record<string, string>)[
				mplan.old_proxy ?? ''
			] ?? ''}
		<h2>Что сделаем</h2>
		<MarkRows>
			<MarkRow mark="1"
				>Поставим Wynd {mplan.version}{dataOnly ? ' и Caddy' : ''} на новый сервер
				<span class="ins-dim">— пока без данных и не запуская</span></MarkRow
			>
			<MarkRow mark="2"
				>Остановим сайт на старом <span class="ins-dim">— с этой минуты он не открывается</span
				></MarkRow
			>
			<MarkRow mark="3"
				>Перенесём {dataOnly ? 'данные' : 'папку'} Wynd на новый сервер
				<span class="ins-dim">— {gb(mplan.size)}, напрямую со старого, мимо этого компьютера</span
				></MarkRow
			>
			<MarkRow mark="4"
				>Вы смените адрес <span class="ins-dim"
					>— у записи A для {mplan.domain} поставите {mplan.new_ip}</span
				></MarkRow
			>
			<MarkRow mark="5"
				>{dataOnly
					? 'Запустим Wynd на новом, получим сертификат сайта и проверим сайт'
					: 'Запустим Wynd на новом и проверим сайт'}</MarkRow
			>
		</MarkRows>
		{#if dataOnly && oldProxy}
			<p class="ins-sub mt">
				Не тронем на старом: {oldProxy} и его сертификат.
			</p>
		{/if}
		{#if !mplan.foreign}
			<Label>Старый сервер</Label>
			<Radio name="mold" value={false} bind:group={mRemoveOld}>
				Оставить Wynd остановленным <span class="ins-dim"
					>— убрать потом, когда убедитесь, что всё на месте</span
				>
			</Radio>
			<Radio name="mold" value={true} bind:group={mRemoveOld}>
				Убрать Wynd после переезда <span class="ins-dim"
					>— со старого сервера уйдут его данные{dataOnly
						? ''
						: '; сертификат переедет на новый'}</span
				>
			</Radio>
		{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void attempt(() => startMove())}>Переехать</Button>
			<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
		</FootBar>
	{:else if screen === 'move' && progress?.failure}
		<h2>{progress.failure.message}</h2>
		{#if progress.failure.advice}<Notice>{progress.failure.advice}</Notice>{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void attempt(() => startMove())}>Повторить</Button>
			<Button
				variant="ghost"
				loading={mBusy}
				loadingLabel="Отменяем переезд…"
				onclick={() => void attempt(() => cancelMove())}>Отменить переезд</Button
			>
			<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}
				>подробности</TextButton
			>
		</FootBar>
		{#if showLog}<CommandLog text={logText} />{/if}
	{:else if screen === 'move'}
		<!-- Кадр 9в. -->
		{@const waitingDNS = progress?.steps?.some((s) => s.id === 'dns' && s.status === 'running')}
		<h2>Переезжаем</h2>
		<StepList meter class="ins-stage-steps" steps={progress?.steps ?? []} />
		{#if waitingDNS && mplan}
			<Notice>
				<div class="ins-dim">
					В панели, где покупали адрес {mplan.domain}, поменяйте у записи A адрес сервера на этот:
				</div>
				<CopyLine
					text={mplan.new_ip}
					copied={copiedLine === mplan.new_ip}
					oncopy={() => void copyLine(mplan?.new_ip ?? '')}
				/>
				<div class="ins-dim">Мастерская проверяет сама каждую минуту — окно можно свернуть.</div>
			</Notice>
			<FootBar>
				<Button variant="ghost" onclick={() => void api.MoveCheckNow()}>Проверить сейчас</Button>
				<TextButton class="link under ins-push" onclick={() => void attempt(() => cancelMove())}
					>Отменить переезд</TextButton
				>
			</FootBar>
		{:else}
			<FootBar>
				<TextButton class="link under" onclick={() => (showLog = !showLog)}
					>показать, что выполняется</TextButton
				>
				<span class="ins-dim ins-push">Окно можно свернуть</span>
			</FootBar>
		{/if}
		{#if showLog}<CommandLog text={logText} />{/if}
	{:else if screen === 'mdone' && mplan}
		<!-- Кадр 9г. -->
		<h2>Wynd на новом сервере</h2>
		<p class="ins-sub">
			Проверка прошла: {mplan.domain} открывается с сервера {mplan.new_ip}.
		</p>
		<Notice>
			{mRemoveOld
				? `Wynd со старого сервера ${mplan.old_ip} убран.`
				: `Старый сервер ${mplan.old_ip}: Wynd остановлен, данные на месте.${
						mplan.foreign
							? ''
							: ' Когда убедитесь, что на новом всё есть, уберите его со страницы старого сервера.'
					}`}
		</Notice>
		<FootBar>
			<Button variant="colored" onclick={() => void api.OpenLink(`https://${mplan?.domain}`)}>
				Открыть {mplan.domain}
			</Button>
			<Button variant="ghost" onclick={() => void attempt(() => adoptNew())}>К серверу</Button>
		</FootBar>
	{:else if screen === 'fingerprint'}
		<h2>
			{changedFingerprint ? 'У сервера другой отпечаток' : 'Это ваш сервер?'}
		</h2>
		<p class="ins-sub">
			{#if changedFingerprint}Подключение остановлено. Сверьте новый отпечаток в панели хостинга.
				Если вы не переустанавливали сервер, не доверяйте новому ключу.{:else}Подключаемся к {host.trim()}
				впервые. Сервер назвал свой отпечаток — сверьте его с письмом хостинга или панелью, если там он
				есть.{/if}
		</p>
		{#if changedFingerprint}<Label>Сохранённый отпечаток</Label><Notice mono
				>{previousFingerprint}</Notice
			><Label>Новый отпечаток</Label>{/if}<Notice mono>{fingerprint}</Notice>
		<p class="ins-sub mt">
			Если отпечатка негде взять — это нормально для нового сервера. Мы запомним его, и при
			следующем подключении подмену заметим.
		</p>
		<FootBar>
			<Button
				variant="colored"
				loading={busy}
				loadingLabel="Подключаемся…"
				onclick={() => void attempt(() => confirmHost())}
			>
				{changedFingerprint ? 'Проверил, доверять новому' : 'Да, подключиться'}
			</Button>
			<Button variant="ghost" disabled={busy} onclick={() => void attempt(() => toConnect())}
				>Отмена</Button
			>
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
				<TextButton
					class="link under"
					onclick={() => void attempt(() => chooseBuild(!plan?.build))}
				>
					{plan.build ? 'Скачать готовый' : 'Собрать самому'}
				</TextButton>
			</p>
		{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void attempt(() => install())}>Установить</Button>
			<Button variant="ghost" onclick={() => (screen = 'inspect')}>Назад</Button>
			<span class="ins-dim">{plan.duration}</span>
		</FootBar>
	{:else if screen === 'install' && progress?.failure}
		<h2>{progress.failure.message}</h2>
		<p class="ins-sub">{progress.failure.advice}</p>
		{#if progress.steps?.some((s) => s.status === 'done')}
			<Notice>Сделанное раньше осталось на месте — повтор начнётся с этого шага.</Notice>
		{/if}
		{#if askRollback}
			{@render removeBox('Да, откатить', '')}
		{:else}
			<FootBar>
				<Button variant="colored" onclick={() => void attempt(() => install())}>Повторить</Button>
				<Button variant="ghost" onclick={() => (askRollback = true)}>Откатить установку</Button>
				<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
			</FootBar>
		{/if}
		{#if showLog}
			<CommandLog text={logText} />
		{/if}
	{:else if screen === 'install'}
		<h2>Ставим</h2>
		<StepList meter class="ins-stage-steps" steps={progress?.steps ?? []} />
		<FootBar>
			<TextButton class="link under" onclick={() => (showLog = !showLog)}>
				показать, что выполняется
			</TextButton>
			<span class="ins-dim ins-push">Окно можно свернуть</span>
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
				По ней вы станете администратором сервера и создадите первый круг. Никому её не передавайте.
			</p>
			<FootBar>
				<Button variant="colored" onclick={() => void api.OpenLink(progress?.link ?? '')}>
					Открыть в браузере
				</Button>
				<Button variant="ghost" onclick={() => void copyLink()}>
					{copied ? 'Скопировано' : 'Скопировать'}
				</Button>
				<Button variant="ghost" onclick={() => void attempt(toServer)}>К серверу</Button>
				<TextButton class="link under ins-push" onclick={() => (askRollback = true)}>
					Убрать Wynd
				</TextButton>
			</FootBar>
		{:else}
			<Notice>
				Этот сервер уже настроен: администратор у него есть, ссылки первого запуска нет. Откройте
				сайт и войдите как обычно.
			</Notice>
			<FootBar>
				<Button variant="colored" onclick={() => void api.OpenLink(progress?.site ?? '')}>
					Открыть {plan?.domain}
				</Button>
				<Button variant="ghost" onclick={() => void attempt(toServer)}>К серверу</Button>
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
			<Button variant="colored" onclick={() => void attempt(() => toConnect())}
				>Подключиться ещё раз</Button
			>
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
					<CopyLine
						text={block.copy}
						copied={copiedLine === block.copy}
						oncopy={() => void copyLine(block.copy ?? '')}
					/>
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
				<Button variant="colored" disabled={busy} onclick={() => void attempt(() => toConnect())}>
					Другой сервер
				</Button>
				<Button
					variant="ghost"
					loading={busy}
					loadingLabel="Осматриваем…"
					onclick={() => void attempt(() => inspect())}
				>
					Проверить снова
				</Button>
			{:else}
				<Button
					variant="colored"
					loading={busy}
					loadingLabel="Осматриваем…"
					onclick={() => void attempt(() => inspect())}
				>
					Проверить снова
				</Button>
				<Button variant="ghost" disabled={busy} onclick={() => void attempt(() => toConnect())}>
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
					{f.text}{#if f.plan}&nbsp;— <span class="ins-dim"
							>{f.plan}{f.id === 'wynd' ? '.' : ''}</span
						>{/if}
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
				void attempt(() => checkDomain());
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
				<Button
					variant="ghost"
					loading={checkingDomain}
					loadingLabel="Проверяем…"
					onclick={() => void attempt(() => checkDomain())}
				>
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
				loadingLabel="Составляем план…"
				onclick={() => void attempt(() => toPlan())}
			>
				Дальше — план
			</Button>
			<Button variant="ghost" onclick={() => void attempt(() => toConnect())}>Другой сервер</Button>
			{#if !domainOk}
				<span class="ins-dim">Сначала укажите адрес сайта</span>
			{/if}
			<TextButton class="link under" onclick={() => (showRaw = true)}>
				подробнее, что проверили
			</TextButton>
		</FootBar>
	{/if}
	{#if !occupied}
	<FootBar class="ins-profile-tools"
		><TextButton
			class="link under"
			disabled={occupied || moving || Boolean(profile.reserved_by)}
			onclick={onedit}>Изменить доступ</TextButton
		><TextButton
			class="link under"
			disabled={occupied || moving || Boolean(profile.reserved_by)}
			onclick={onforget}>Убрать из списка</TextButton
		>{#if screen === 'server' && page && !page.foreign && !page.state.move && !askRollback}<TextButton
				class="link under"
				disabled={occupied}
				onclick={() => (askRollback = true)}>Убрать Wynd с сервера</TextButton
			>{/if}
	</FootBar>
	{/if}
</main>
{#if showRaw && inspection}
	<RawSheet
		parts={rawParts}
		copied={copiedRaw}
		oncopy={() => void copyRaw()}
		onclose={() => (showRaw = false)}
	/>
{/if}
