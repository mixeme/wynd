import { readCachedSnapshot, writeCachedSnapshot } from '$lib/api/snapshots';
import { getSnapshotEntries } from '$lib/idb/db';

/**
 * «В круге есть кто-то ещё» — от этого зависит вкладка «Отклики» (3.17).
 *
 * Сведения о круге приходят при каждом открытии, поэтому признак кладём в
 * снимок здесь: следующее открытие с улочки покажет вкладку в первом кадре,
 * не заставляя открывать «Настройки». Снимок настроек для этого не годится —
 * он есть только у кругов, где их открывали.
 */
interface OthersSnapshot {
	has_others: boolean;
}

export async function readHasOthers(
	origin: string,
	circleId: string
): Promise<boolean | undefined> {
	const snap = await readCachedSnapshot<OthersSnapshot>(origin, 'meta', circleId);
	return snap?.has_others;
}

export async function writeHasOthers(
	origin: string,
	circleId: string,
	value: boolean
): Promise<void> {
	await writeCachedSnapshot(origin, 'meta', circleId, { has_others: value });
}

/** Признаки всех кругов сервера для строк улочки: один проход по снимкам. */
export async function readHasOthersMap(origin: string): Promise<Map<string, boolean>> {
	const prefix = `${origin}:meta:`;
	const out = new Map<string, boolean>();
	for (const { key, data } of await getSnapshotEntries(prefix)) {
		const value = (data as OthersSnapshot | undefined)?.has_others;
		if (typeof value === 'boolean') out.set(key.slice(prefix.length), value);
	}
	return out;
}
