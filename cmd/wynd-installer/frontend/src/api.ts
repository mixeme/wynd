// То, что отдаёт Go (internal/installer). Wails вешает методы App на
// window.go.main.App; в `wails dev` они есть и в обычном браузере.
export type Level = 'ok' | 'note' | 'block';

export interface ConnectInput {
	host: string;
	user: string;
	password: string;
	use_keys: boolean;
	remember: boolean;
	/** «Дальше входить по ключу» (кадр 1). */
	make_key: boolean;
}

export interface ConnectResult {
	status: 'connected' | 'unknown_host' | 'error';
	host: string;
	fingerprint?: string;
	message?: string;
	advice?: string;
	key_note?: string;
}

export interface Finding {
	id: string;
	level: Level;
	text: string;
	plan?: string;
	advice?: string;
	/** Строка, которую человек вставит на сервере; after — текст под ней. */
	copy?: string;
	after?: string;
	/** Повторная проверка ничего не изменит: главная кнопка — «Другой сервер». */
	final?: boolean;
}

export interface Report {
	findings: Finding[] | null;
	addresses: string[] | null;
	proxy?: string;
	raw: string;
}

export interface InspectResult {
	ok: boolean;
	message?: string;
	advice?: string;
	report: Report;
}

export interface DomainCheck {
	domain: string;
	level: Level;
	text: string;
	advice?: string;
}

export interface Plan {
	ok: boolean;
	message?: string;
	advice?: string;
	domain: string;
	version: string;
	install: string[] | null;
	change: string[] | null;
	keep: string[] | null;
	duration: string;
	build: boolean;
	canBuild: boolean;
	unpublished: boolean;
}

export interface StepState {
	id: string;
	title: string;
	status: 'pending' | 'running' | 'done' | 'failed';
	note?: string;
}

export interface LogEntry {
	command: string;
	output: string;
}

export interface Failure {
	step: string;
	message: string;
	advice: string;
	unpublished?: boolean;
}

export interface Progress {
	steps: StepState[] | null;
	log: LogEntry[] | null;
	running: boolean;
	done: boolean;
	failure?: Failure;
	site?: string;
	link?: string;
}

export interface RollbackResult {
	ok: boolean;
	message?: string;
	advice?: string;
	log: LogEntry[] | null;
	cert?: 'revoked' | 'failed';
	/** Чужой веб-сервер настройку не перечитал: ошибка была до удаления, появилась после него или не вышло перечитать. */
	stale?: 'before' | 'after' | 'reload';
	report?: Report;
	plan?: Plan;
}

export interface UpdateMark {
	from: string;
	to: string;
	ref: string;
	stopped: boolean;
	start_after: boolean;
	started: string;
}

export interface ServerState {
	installed: boolean;
	version?: string;
	image?: string;
	domain?: string;
	running: boolean;
	container: boolean;
	stopped_at?: string;
	free_bytes: number;
	update?: UpdateMark;
}

/** Страница сервера, где Wynd уже стоит (пульт, кадры 7а–7е). */
export interface ServerPage {
	ok: boolean;
	message?: string;
	advice?: string;
	state: ServerState;
	latest?: string;
	copy_bytes: number;
	live_copy: boolean;
	can_build: boolean;
}

export interface ServiceResult {
	ok: boolean;
	message?: string;
	advice?: string;
	log: LogEntry[] | null;
	page: ServerPage;
}

export interface UpdateOptions {
	stop_for_copy: boolean;
	start_after: boolean;
	build: boolean;
}

/** Архивы на этом компьютере (кадры 11а–11г). */
export interface ArchiveSummary {
	circle_name: string;
	from?: string;
	to?: string;
	posts: number;
	duplicates: number;
	bytes: number;
}

export interface ArchiveFile {
	path: string;
	name: string;
	ok: boolean;
	error?: string;
	summary: ArchiveSummary;
}

export interface MergeInfo {
	files: ArchiveFile[] | null;
	ok: boolean;
	summary: ArchiveSummary;
	suggest?: string;
}

export interface SplitInfo {
	file: ArchiveFile;
	years: { year: number; summary: ArchiveSummary }[] | null;
}

export interface ArchiveResult {
	ok: boolean;
	message?: string;
	files?: string[] | null;
}

export interface FullInfo {
	ok: boolean;
	name?: string;
	bytes?: number;
	encrypted?: boolean;
}

export interface LocalBackupPlan {
	ok: boolean;
	message?: string;
	total: number;
	fetch: number;
	prev?: string;
	encrypted?: boolean;
}

export interface BackupOptions {
	folder: string;
	site: 'keep' | 'pause' | 'stop';
	password: string;
}

export interface MovePlan {
	ok: boolean;
	message?: string;
	advice?: string;
	domain: string;
	version: string;
	new_ip: string;
	old_ip: string;
	size: number;
}

/** Новая Мастерская (кадры 10а–10в). */
export interface SelfUpdate {
	available: boolean;
	version?: string;
	notes?: string[] | null;
	attestation?: string;
}

interface Backend {
	HasSavedPassword(host: string, user: string): Promise<boolean>;
	Connect(input: ConnectInput): Promise<ConnectResult>;
	ConfirmHost(): Promise<ConnectResult>;
	Inspect(): Promise<InspectResult>;
	CheckDomain(domain: string): Promise<DomainCheck>;
	MakePlan(domain: string): Promise<Plan>;
	ChooseBuild(on: boolean): Promise<Plan>;
	StartInstall(): Promise<Progress>;
	InstallProgress(): Promise<Progress>;
	Rollback(keepData: boolean): Promise<RollbackResult>;
	OpenLink(url: string): Promise<void>;
	CopyText(text: string): Promise<boolean>;
	Disconnect(): Promise<void>;
	Version(): Promise<string>;
	ServerPage(): Promise<ServerPage>;
	Service(action: 'start' | 'stop' | 'restart'): Promise<ServiceResult>;
	StartUpdate(opt: UpdateOptions): Promise<Progress>;
	ReturnPrevious(): Promise<ServiceResult>;
	PickArchives(): Promise<string[] | null>;
	PickArchive(full: boolean): Promise<string>;
	PickSave(name: string): Promise<string>;
	PickFolder(): Promise<string>;
	MergeInfo(paths: string[]): Promise<MergeInfo>;
	Merge(paths: string[], out: string): Promise<ArchiveResult>;
	SplitInfo(path: string): Promise<SplitInfo>;
	Split(path: string, years: number[], from: string, to: string, dir: string): Promise<ArchiveResult>;
	FullInfo(path: string): Promise<FullInfo>;
	Readable(path: string, password: string, out: string): Promise<ArchiveResult>;
	ShowInFolder(path: string): Promise<void>;
	PlanBackup(folder: string): Promise<LocalBackupPlan>;
	StartBackup(opt: BackupOptions): Promise<Progress>;
	DefaultBackupFolder(domain: string): Promise<string>;
	MoveConnect(input: ConnectInput): Promise<ConnectResult>;
	MoveConfirmHost(): Promise<ConnectResult>;
	MovePlan(): Promise<MovePlan>;
	StartMove(removeOld: boolean): Promise<Progress>;
	MoveCheckNow(): Promise<void>;
	CancelMove(): Promise<ServiceResult>;
	MoveAdopt(): Promise<ServerPage>;
	CheckSelfUpdate(): Promise<SelfUpdate>;
	ApplySelfUpdate(): Promise<string>;
	ReleasesPage(): Promise<void>;
}

export function backend(): Backend {
	return (window as unknown as { go: { main: { App: Backend } } }).go.main.App;
}
