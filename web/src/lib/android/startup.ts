/** Протокол только встроенной стартовой страницы; серверный интерфейс его не получает. */
import type { InterfaceServer } from './interfaces';

interface StartupPort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}
export interface StartupReply {
	offline?: boolean;
	selected?: InterfaceServer | null;
	preview?: InterfaceServer;
}
function port(): StartupPort | undefined {
	if (typeof window === 'undefined') return;
	return (window as unknown as { __wyndStartup?: StartupPort }).__wyndStartup;
}
export function startupAvailable(): boolean {
	return typeof port()?.postMessage === 'function';
}
let sequence = 0;
const pending = new Map<
	number,
	{
		resolve: (reply: StartupReply) => void;
		reject: (error: Error) => void;
		timer: ReturnType<typeof setTimeout>;
	}
>();
let connected: StartupPort | undefined;
export function startupRequest(
	op: 'state' | 'check' | 'open' | 'retry' | 'cancel',
	address?: string
): Promise<StartupReply> {
	const target = port();
	if (!target) return Promise.reject(new Error('Startup unavailable'));
	if (connected !== target) {
		connected = target;
		target.onmessage = ({ data }) => {
			try {
				const reply = JSON.parse(data);
				const callback = pending.get(reply.id);
				if (!callback) return;
				pending.delete(reply.id);
				clearTimeout(callback.timer);
				if (reply.protocol !== 1 || reply.error) callback.reject(new Error('Startup failed'));
				else callback.resolve(reply);
			} catch {
				/* Повреждённый ответ не меняет экран или выбранный источник. */
			}
		};
	}
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			pending.delete(id);
			reject(new Error('Startup timeout'));
		}, 30000);
		pending.set(id, { resolve, reject, timer });
		try {
			target.postMessage(JSON.stringify({ id, op, ...(address === undefined ? {} : { address }) }));
		} catch {
			pending.delete(id);
			clearTimeout(timer);
			reject(new Error('Startup unavailable'));
		}
	});
}
