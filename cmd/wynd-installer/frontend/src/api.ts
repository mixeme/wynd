// То, что отдаёт Go (internal/installer). Wails вешает методы App на
// window.go.main.App; в `wails dev` они есть и в обычном браузере.
export type Level = 'ok' | 'note' | 'block';

export interface ConnectInput {
	host: string;
	user: string;
	password: string;
	use_keys: boolean;
	remember: boolean;
}

export interface ConnectResult {
	status: 'connected' | 'unknown_host' | 'error';
	host: string;
	fingerprint?: string;
	message?: string;
	advice?: string;
}

export interface Finding {
	id: string;
	level: Level;
	text: string;
	plan?: string;
	advice?: string;
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
}

interface Backend {
	HasSavedPassword(host: string, user: string): Promise<boolean>;
	Connect(input: ConnectInput): Promise<ConnectResult>;
	ConfirmHost(): Promise<ConnectResult>;
	Inspect(): Promise<InspectResult>;
	CheckDomain(domain: string): Promise<DomainCheck>;
	MakePlan(domain: string): Promise<Plan>;
	StartInstall(): Promise<Progress>;
	InstallProgress(): Promise<Progress>;
	Rollback(keepData: boolean): Promise<RollbackResult>;
	OpenLink(url: string): Promise<void>;
	CopyText(text: string): Promise<boolean>;
	Disconnect(): Promise<void>;
}

export function backend(): Backend {
	return (window as unknown as { go: { main: { App: Backend } } }).go.main.App;
}
