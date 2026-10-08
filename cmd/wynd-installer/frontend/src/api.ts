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

interface Backend {
	HasSavedPassword(host: string, user: string): Promise<boolean>;
	Connect(input: ConnectInput): Promise<ConnectResult>;
	ConfirmHost(): Promise<ConnectResult>;
	Inspect(): Promise<InspectResult>;
	CheckDomain(domain: string): Promise<DomainCheck>;
	Disconnect(): Promise<void>;
}

export function backend(): Backend {
	return (window as unknown as { go: { main: { App: Backend } } }).go.main.App;
}
