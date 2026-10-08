import { SvelteMap, SvelteSet } from 'svelte/reactivity';
import { normalizeOrigin } from '$lib/api/client';
import {
	drainQueueFor,
	listQueueItems,
	subscribeQueue,
	subscribeQueueProgress,
	type QueueRecordWithId
} from '$lib/queue/queue';

/**
 * Видно ли, есть ли связь (план 46, C25; кадры 7.5–7.9). На связи знака нет.
 * Нет сети — одно состояние на всё приложение; сервер молчит — своё у каждого
 * сервера. Оба показываются не сразу: короткий обрыв не должен мигать.
 */
export const OFFLINE_GRACE_MS = 4000;
export const SERVER_GRACE_MS = 8000;

/** «Соединение установлено — отправляем…» (7.8): что ещё ждёт и общий ход. */
export interface Recovery {
	posts: number;
	comments: number;
	/** Доля отправленного от того, что ждало в момент возврата связи, 0..1. */
	progress: number;
}

/** Что показать полосой под шапкой; на связи — ничего. */
export type ConnectionNotice =
	| { kind: 'offline' }
	| { kind: 'down' }
	| ({ kind: 'sending' } & Recovery);

interface RecoveryRun {
	total: number;
	ids: Set<number>;
}

let offline = $state(false);
const down = new SvelteSet<string>();
const recovery = new SvelteMap<string, Recovery>();

let offlineTimer: ReturnType<typeof setTimeout> | undefined;
const downTimers = new Map<string, ReturnType<typeof setTimeout>>();
const runs = new Map<string, RecoveryRun>();
const fractions = new Map<number, number>();
let stopQueueWatch: (() => void) | undefined;

function online(): boolean {
	return typeof navigator === 'undefined' || navigator.onLine;
}

export function isOffline(): boolean {
	return offline;
}

export function isServerDown(origin: string): boolean {
	return down.has(normalizeOrigin(origin));
}

export function recoveryOf(origin: string): Recovery | undefined {
	return recovery.get(normalizeOrigin(origin));
}

/**
 * Полоса для экрана. С `origin` — экран круга: его сервер и его очередь.
 * Без него — улочка: только «нет сети», молчащий сервер там виден у своих
 * кругов (7.9).
 */
export function connectionNotice(origin?: string): ConnectionNotice | undefined {
	if (offline) return { kind: 'offline' };
	if (origin === undefined) return undefined;
	const key = normalizeOrigin(origin);
	if (down.has(key)) return { kind: 'down' };
	const sending = recovery.get(key);
	return sending ? { kind: 'sending', ...sending } : undefined;
}

function clearDownTimer(key: string) {
	const timer = downTimers.get(key);
	if (timer) clearTimeout(timer);
	downTimers.delete(key);
}

function armDownTimer(key: string) {
	clearDownTimer(key);
	downTimers.set(
		key,
		setTimeout(() => {
			downTimers.delete(key);
			if (!online()) return;
			down.add(key);
			endRecovery(key);
		}, SERVER_GRACE_MS)
	);
}

/** Поток синхронизации открыт: сервер отвечает. */
export function reportServerUp(origin: string): void {
	const key = normalizeOrigin(origin);
	clearDownTimer(key);
	if (down.delete(key)) void startRecovery([key]);
}

/**
 * Сервер не ответил или поток оборвался. Молчащим он считается, если не
 * ответил и через SERVER_GRACE_MS. Без сети это не про сервер — молчим.
 */
export function reportServerSilent(origin: string): void {
	if (!online()) return;
	const key = normalizeOrigin(origin);
	if (down.has(key) || downTimers.has(key)) return;
	armDownTimer(key);
}

/** Вышли с сервера — следить не за чем. */
export function forgetServer(origin: string): void {
	const key = normalizeOrigin(origin);
	clearDownTimer(key);
	down.delete(key);
	endRecovery(key);
}

function goOffline() {
	offline = true;
	// Без сети молчат все: полоса одна, про сеть, а не про каждый сервер.
	for (const key of [...downTimers.keys()]) clearDownTimer(key);
	down.clear();
	for (const key of [...runs.keys()]) endRecovery(key);
}

function onOffline() {
	if (offline || offlineTimer) return;
	offlineTimer = setTimeout(() => {
		offlineTimer = undefined;
		if (!online()) goOffline();
	}, OFFLINE_GRACE_MS);
}

function onOnline() {
	if (offlineTimer) clearTimeout(offlineTimer);
	offlineTimer = undefined;
	if (!offline) return;
	offline = false;
	void startRecovery();
}

/** Вернулись из фона: таймеры там стояли, отсчёт молчания начинается заново. */
function onVisible() {
	if (document.visibilityState !== 'visible') return;
	for (const key of [...downTimers.keys()]) armDownTimer(key);
}

/** Корневой макет зовёт один раз. */
export function initConnection(): () => void {
	if (!online()) goOffline();
	window.addEventListener('offline', onOffline);
	window.addEventListener('online', onOnline);
	document.addEventListener('visibilitychange', onVisible);
	return () => {
		window.removeEventListener('offline', onOffline);
		window.removeEventListener('online', onOnline);
		document.removeEventListener('visibilitychange', onVisible);
	};
}

function waiting(item: QueueRecordWithId): boolean {
	return item.state !== 'failed' && (item.type === 'post' || item.type === 'comment');
}

function publish(key: string, items: QueueRecordWithId[]) {
	const run = runs.get(key);
	if (!run) return;
	const left = items.filter((item) => waiting(item) && run.ids.has(item.id));
	if (!left.length) {
		endRecovery(key);
		return;
	}
	let sending = 0;
	for (const item of left) sending += fractions.get(item.id) ?? 0;
	recovery.set(key, {
		posts: left.filter((item) => item.type === 'post').length,
		comments: left.filter((item) => item.type === 'comment').length,
		progress: Math.min(1, (run.total - left.length + sending) / run.total)
	});
}

async function refreshRecovery() {
	if (!runs.size) return;
	const items = await listQueueItems();
	for (const key of [...runs.keys()]) {
		publish(
			key,
			items.filter((item) => normalizeOrigin(item.origin) === key)
		);
	}
}

function endRecovery(key: string) {
	runs.delete(key);
	recovery.delete(key);
	if (!runs.size) {
		stopQueueWatch?.();
		stopQueueWatch = undefined;
		fractions.clear();
	}
}

/**
 * Связь вернулась, а в очереди ждёт неотправленное — полоса говорит, что оно
 * уходит. Ждать нечего — полоса просто пропадает (7.8). `keys` — серверы,
 * которые ожили; без них — все, у кого что-то ждёт.
 */
async function startRecovery(keys?: string[]) {
	let items: QueueRecordWithId[];
	try {
		items = await listQueueItems();
	} catch {
		return;
	}
	const byOrigin = new Map<string, QueueRecordWithId[]>();
	for (const item of items) {
		if (!waiting(item)) continue;
		const key = normalizeOrigin(item.origin);
		if (keys && !keys.includes(key)) continue;
		byOrigin.set(key, [...(byOrigin.get(key) ?? []), item]);
	}
	if (!byOrigin.size) return;
	if (!stopQueueWatch) {
		const stopList = subscribeQueue(() => void refreshRecovery());
		const stopProgress = subscribeQueueProgress((id, fraction) => {
			if (fraction === undefined) fractions.delete(id);
			else fractions.set(id, fraction);
			void refreshRecovery();
		});
		stopQueueWatch = () => {
			stopList();
			stopProgress();
		};
	}
	for (const [key, mine] of byOrigin) {
		if (offline || down.has(key)) continue;
		const run = { total: mine.length, ids: new Set(mine.map((item) => item.id)) };
		runs.set(key, run);
		publish(key, mine);
		// Слив кончился, а что-то осталось (ждёт повтора через паузу) — полоса
		// не висит: у записи в ленте своя подпись «в очереди».
		void drainQueueFor(key).finally(() => {
			if (runs.get(key) === run) endRecovery(key);
		});
	}
}
