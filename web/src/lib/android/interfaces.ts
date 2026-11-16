/** Источники интерфейса независимы от сессий; протокол только выбранного Android origin. */
interface InterfacePort {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}
export interface InterfaceServer {
	origin: string;
	name: string;
}
export interface InterfaceCatalog {
	selected: string;
	servers: InterfaceServer[];
}
export class InterfaceError extends Error {
	constructor(readonly code: string) {
		super(`Interface request: ${code}`);
	}
}
interface Reply {
	catalog?: unknown;
	preview?: unknown;
}
let sequence = 0;
const connections = new WeakMap<
	InterfacePort,
	Map<
		number,
		{
			resolve: (value: Reply) => void;
			reject: (error: Error) => void;
			timer: ReturnType<typeof setTimeout>;
		}
	>
>();
function port(): InterfacePort | undefined {
	if (typeof window === 'undefined') return;
	const value = (window as unknown as { __wyndInterfaces?: InterfacePort }).__wyndInterfaces;
	return typeof value?.postMessage === 'function' ? value : undefined;
}
export function androidInterfacesAvailable(): boolean {
	return !!port();
}
function server(value: unknown): InterfaceServer {
	if (!value || typeof value !== 'object') throw new InterfaceError('invalid');
	const item = value as InterfaceServer;
	if (typeof item.origin !== 'string' || typeof item.name !== 'string' || item.name.length > 512)
		throw new InterfaceError('invalid');
	const url = new URL(item.origin);
	if (!['http:', 'https:'].includes(url.protocol) || url.origin !== item.origin)
		throw new InterfaceError('invalid');
	return { origin: item.origin, name: item.name };
}
function catalog(value: unknown): InterfaceCatalog {
	if (!value || typeof value !== 'object') throw new InterfaceError('invalid');
	const data = value as InterfaceCatalog;
	if (!Array.isArray(data.servers) || data.servers.length > 32) throw new InterfaceError('invalid');
	const servers = data.servers.map(server);
	if (
		!servers.some((item) => item.origin === data.selected) ||
		new Set(servers.map((item) => item.origin)).size !== servers.length
	)
		throw new InterfaceError('invalid');
	return { selected: data.selected, servers };
}
function request(op: string, value: Record<string, string> = {}): Promise<Reply> {
	const target = port();
	if (!target) return Promise.reject(new InterfaceError('unavailable'));
	let callbacks = connections.get(target);
	if (!callbacks) {
		callbacks = new Map();
		connections.set(target, callbacks);
		const pending = callbacks;
		target.onmessage = ({ data }) => {
			try {
				const reply = JSON.parse(data),
					callback = pending.get(reply.id);
				if (!callback) return;
				pending.delete(reply.id);
				clearTimeout(callback.timer);
				if (reply.protocol !== 1) callback.reject(new InterfaceError('invalid'));
				else if (reply.error)
					callback.reject(new InterfaceError(reply.error === 'busy' ? 'busy' : 'failed'));
				else callback.resolve(reply);
			} catch {
				/* Повреждённый ответ не меняет источник. */
			}
		};
	}
	const pending = callbacks;
	return new Promise((resolve, reject) => {
		const id = ++sequence;
		const timer = setTimeout(() => {
			pending.delete(id);
			reject(new InterfaceError('timeout'));
		}, 25_000);
		pending.set(id, { resolve, reject, timer });
		try {
			target.postMessage(JSON.stringify({ id, op, ...value }));
		} catch (error) {
			pending.delete(id);
			clearTimeout(timer);
			reject(error instanceof Error ? error : new InterfaceError('failed'));
		}
	});
}
export async function loadInterfaces(): Promise<InterfaceCatalog> {
	return catalog((await request('list')).catalog);
}
export async function previewInterface(address: string): Promise<InterfaceServer> {
	return server((await request('preview', { address })).preview);
}
export async function addInterface(address: string): Promise<InterfaceCatalog> {
	return catalog((await request('add', { address })).catalog);
}
export async function removeInterface(origin: string): Promise<InterfaceCatalog> {
	return catalog((await request('remove', { origin })).catalog);
}
export async function selectInterface(origin: string): Promise<InterfaceCatalog> {
	return catalog((await request('select', { origin })).catalog);
}
