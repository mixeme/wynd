/**
 * Системный лист «Поделиться» в приложении Android (C159). В WebView нет
 * navigator.share, поэтому страница просит оболочку показать лист. Без моста
 * (браузер, старый APK) вызывающий код сам решает, что делать, — обычно
 * копирует ссылку.
 */
interface SharePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}

interface Pending {
	resolve: (shared: boolean) => void;
	timer: ReturnType<typeof setTimeout>;
}

let sequence = 0;
const connections = new WeakMap<SharePort, Map<number, Pending>>();

function port(): SharePort | undefined {
	if (typeof window === 'undefined') return;
	const value = (window as unknown as { __wyndShare?: SharePort }).__wyndShare;
	return typeof value?.postMessage === 'function' ? value : undefined;
}

export function androidShareAvailable(): boolean {
	return !!port();
}

/** Показать системный лист; true — лист открыт, false — моста нет или отказ. */
export function androidShare(url: string, title?: string): Promise<boolean> {
	const target = port();
	if (!target) return Promise.resolve(false);
	let pending = connections.get(target);
	if (!pending) {
		pending = new Map();
		connections.set(target, pending);
		const callbacks = pending;
		target.onmessage = ({ data }) => {
			try {
				const reply = JSON.parse(data) as { id?: number; shared?: boolean };
				const callback = typeof reply.id === 'number' ? callbacks.get(reply.id) : undefined;
				if (!callback) return;
				callbacks.delete(reply.id as number);
				clearTimeout(callback.timer);
				callback.resolve(reply.shared === true);
			} catch {
				/* Повреждённый ответ не завершает запрос. */
			}
		};
	}
	const callbacks = pending;
	return new Promise((resolve) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			callbacks.delete(id);
			resolve(false);
		}, 5000);
		callbacks.set(id, { resolve, timer });
		try {
			target.postMessage(JSON.stringify({ id, url, title }));
		} catch {
			clearTimeout(timer);
			callbacks.delete(id);
			resolve(false);
		}
	});
}
