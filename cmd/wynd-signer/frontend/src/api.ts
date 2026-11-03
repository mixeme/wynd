// То, что отдаёт Go (cmd/wynd-signer/main.go). Wails вешает методы App на
// window.go.main.App.

export interface State {
	key_path: string;
	key_exists: boolean;
	pub_path: string;
}

export interface Result {
	ok: boolean;
	wrong_password?: boolean;
	message?: string;
	path?: string;
	pub_path?: string;
	published?: boolean;
	url?: string;
	steps?: Step[];
}

export interface Release {
	tag: string;
	version: string;
	draft: boolean;
	published_at?: string;
	signed: boolean;
	url?: string;
}

export interface Releases {
	ok: boolean;
	message?: string;
	releases: Release[] | null;
}

export interface Check {
	tag: string;
	draft: boolean;
	key_in_release: boolean;
	size: number;
	sum_ok: boolean;
	commit?: string;
	attest_ok: boolean;
	notes?: string;
	problem?: string;
	steps: Step[];
}

export interface Step {
	id: string;
	title: string;
	status: 'pending' | 'running' | 'done' | 'failed';
	note?: string;
}

export interface Progress {
	tag: string;
	phase: 'check' | 'sign';
	steps: Step[];
}

export function watchProgress(callback: (progress: Progress) => void): () => void {
	const rt = (window as unknown as {
		runtime?: { EventsOn(name: string, callback: (progress: Progress) => void): () => void };
	}).runtime;
	return rt?.EventsOn('signer:progress', callback) ?? (() => {});
}

interface Backend {
	State(): Promise<State>;
	ChooseKeyPlace(current: string): Promise<string>;
	ChooseKey(): Promise<State>;
	CreateKey(path: string, pass: string): Promise<Result>;
	Releases(): Promise<Releases>;
	CheckRelease(tag: string): Promise<Check>;
	SignRelease(pass: string): Promise<Result>;
	PickExe(): Promise<string>;
	FileSize(path: string): Promise<number>;
	SignFile(path: string, pass: string): Promise<Result>;
	OpenLink(url: string): Promise<void>;
	ShowInFolder(path: string): Promise<void>;
}

export function backend(): Backend {
	return (window as unknown as { go: { main: { App: Backend } } }).go.main.App;
}
