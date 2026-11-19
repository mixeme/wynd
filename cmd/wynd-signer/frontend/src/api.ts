// То, что отдаёт Go (cmd/wynd-signer/main.go). Wails v3 зовётся по имени
// сервиса main.App (те же имена, что в сгенерированных биндингах); адаптер
// написан руками, чтобы проверки фронтенда не требовали CLI wails3.
// mock-стенд (mock/mock.js) по-прежнему подменяет мост через window.go.
import { Call, Events } from '@wailsio/runtime';

export interface State {
	key_path: string;
	key_exists: boolean;
	pub_path: string;
	// key_store_path — файл ключа Android (.p12); key_store_exists — он найден.
	key_store_path: string;
	key_store_exists: boolean;
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
	// detail — готовый текст итога подписи для Android; у exe его нет.
	detail?: string;
}

export interface Release {
	// product — идентификатор программы; name — как она называется в окне,
	// exe — имя файла выпуска. Приходят из описания программы в Go.
	product: string;
	name: string;
	exe: string;
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
	check_id: string;
	request_id: string;
	// product, name и exe — как у Release.
	product: string;
	name: string;
	exe: string;
	// self_update — программа обновляется сама и ждёт в сборке открытый ключ.
	self_update: boolean;
	// publishes — черновик будет опубликован после подписи (решает Go:
	// для Android — только при releaseReady=true в теге).
	publishes: boolean;
	// publish_note — почему публикация не состоится (Android без releaseReady).
	publish_note?: string;
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
	// link — действие в строке шага; приходит из Go вместе с итогом шага.
	link?: { label: string; url: string };
}

export interface Progress {
	check_id: string;
	request_id: string;
	tag: string;
	phase: 'check' | 'sign';
	steps: Step[];
}

export function watchProgress(callback: (progress: Progress) => void): () => void {
	const rt = (window as unknown as {
		runtime?: { EventsOn(name: string, callback: (progress: Progress) => void): () => void };
	}).runtime;
	if (rt?.EventsOn) return rt.EventsOn('signer:progress', callback);
	return Events.On('signer:progress', (event) => callback(event.data as Progress));
}

interface Backend {
	State(): Promise<State>;
	ChooseKeyPlace(current: string): Promise<string>;
	ChooseKey(): Promise<State>;
	// ChooseKeystore — выбор файла ключа Android на экране Android-выпуска.
	ChooseKeystore(): Promise<State>;
	CreateKey(path: string, pass: string): Promise<Result>;
	Releases(): Promise<Releases>;
	CheckRelease(tag: string, requestId: string): Promise<Check>;
	SignRelease(checkId: string, tag: string, requestId: string, pass: string): Promise<Result>;
	PickExe(): Promise<string>;
	FileSize(path: string): Promise<number>;
	SignFile(path: string, pass: string): Promise<Result>;
	OpenLink(url: string): Promise<void>;
	ShowInFolder(path: string): Promise<void>;
}

const native: Backend = {
	State: () => Call.ByName('main.App.State'),
	ChooseKeyPlace: (current) => Call.ByName('main.App.ChooseKeyPlace', current),
	ChooseKey: () => Call.ByName('main.App.ChooseKey'),
	ChooseKeystore: () => Call.ByName('main.App.ChooseKeystore'),
	CreateKey: (path, pass) => Call.ByName('main.App.CreateKey', path, pass),
	Releases: () => Call.ByName('main.App.Releases'),
	CheckRelease: (tag, requestId) => Call.ByName('main.App.CheckRelease', tag, requestId),
	SignRelease: (checkId, tag, requestId, pass) =>
		Call.ByName('main.App.SignRelease', checkId, tag, requestId, pass),
	PickExe: () => Call.ByName('main.App.PickExe'),
	FileSize: (path) => Call.ByName('main.App.FileSize', path),
	SignFile: (path, pass) => Call.ByName('main.App.SignFile', path, pass),
	OpenLink: (url) => Call.ByName('main.App.OpenLink', url),
	ShowInFolder: (path) => Call.ByName('main.App.ShowInFolder', path),
};

export function backend(): Backend {
	const mock = (window as unknown as { go?: { main?: { App?: Backend } } }).go?.main?.App;
	return mock ?? native;
}
