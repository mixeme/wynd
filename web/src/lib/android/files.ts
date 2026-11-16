/** Системное сохранение: выбранный документ, ограниченные части и подтверждение каждой записи. */
interface FilePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}
interface Reply {
	id: number;
	state: 'ready' | 'saved' | 'cancelled' | 'error';
	detail: string;
}

let sequence = 0;
const connections = new WeakMap<
	FilePort,
	Map<number, { resolve: (reply: Reply) => void; reject: (error: Error) => void }>
>();

function port(): FilePort | undefined {
	if (typeof window === 'undefined') return undefined;
	const candidate = (window as unknown as { __wyndFiles?: FilePort }).__wyndFiles;
	return typeof candidate?.postMessage === 'function' ? candidate : undefined;
}

export function androidFilesAvailable(): boolean {
	return !!port();
}

function request(target: FilePort, value: Record<string, unknown>): Promise<Reply> {
	let pending = connections.get(target);
	if (!pending) {
		pending = new Map();
		connections.set(target, pending);
		const callbacks = pending;
		target.onmessage = (event) => {
			try {
				const reply: Reply = JSON.parse(event.data);
				if (
					!Number.isSafeInteger(reply.id) ||
					typeof reply.detail !== 'string' ||
					!['ready', 'saved', 'cancelled', 'error'].includes(reply.state)
				)
					return;
				const callback = callbacks.get(reply.id);
				if (!callback) return;
				callbacks.delete(reply.id);
				if (reply.state === 'error') callback.reject(new Error(`Android export: ${reply.detail}`));
				else callback.resolve(reply);
			} catch {
				/* Посторонний ответ не завершает сохранение. */
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		pending.set(id, { resolve, reject });
		try {
			target.postMessage(JSON.stringify({ ...value, id }));
		} catch (error) {
			pending.delete(id);
			reject(error);
		}
	});
}

export async function saveAndroidBlob(blob: Blob, filename: string): Promise<boolean> {
	const target = port();
	if (!target) return false;
	const begin = await request(target, {
		op: 'begin',
		name: filename,
		mime: blob.type,
		size: blob.size
	});
	if (begin.state === 'cancelled') return true;
	if (begin.state !== 'ready' || !begin.detail) throw new Error('Android export not ready');
	const token = begin.detail;
	let part = 0;
	try {
		// Не arrayBuffer всего видео: максимум 32 KiB в JS и одна неподтверждённая часть.
		for (let offset = 0; offset < blob.size; offset += 32768) {
			const bytes = new Uint8Array(await blob.slice(offset, offset + 32768).arrayBuffer());
			let binary = '';
			for (const byte of bytes) binary += String.fromCharCode(byte);
			const reply = await request(target, {
				op: 'chunk',
				token,
				sequence: part++,
				data: btoa(binary)
			});
			if (reply.state === 'cancelled') return true;
			if (reply.state !== 'ready') throw new Error('Android export chunk rejected');
		}
		const end = await request(target, { op: 'end', token, sequence: part });
		if (!['saved', 'cancelled'].includes(end.state)) throw new Error('Android export incomplete');
		return true;
	} catch (error) {
		try {
			await request(target, { op: 'cancel', token });
		} catch {
			/* Первичная ошибка важнее. */
		}
		throw error;
	}
}

export async function saveAndroidObjectUrl(url: string, filename: string): Promise<boolean> {
	if (!port()) return false;
	if (!url.startsWith(`blob:${location.origin}/`)) throw new Error('Expected local media Blob');
	return saveAndroidBlob(await (await fetch(url)).blob(), filename);
}

/** Архив идёт с сервера прямо в документ на native worker, без памяти/сети WebView. */
export async function saveAndroidDownload(href: string, filename?: string): Promise<boolean> {
	const target = port();
	if (!target) return false;
	const url = new URL(href, location.href);
	const full = /\/full_archive(?:_stream)?\//.test(url.pathname);
	const name = filename ?? (full ? 'wynd-archive.wynd' : 'wynd-archive.zip');
	const result = await request(target, {
		op: 'begin',
		url: url.href,
		name,
		mime: full ? 'application/octet-stream' : 'application/zip'
	});
	if (!['saved', 'cancelled'].includes(result.state))
		throw new Error('Android download incomplete');
	return true;
}
