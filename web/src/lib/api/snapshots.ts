import {
	getSnapshot,
	invalidateSnapshots,
	putSnapshot,
	snapshotKey,
	type InvalidateSnapshotsOptions
} from '$lib/idb/db';

export type SnapshotKind =
	| 'circles'
	| 'feed'
	| 'grid'
	| 'map'
	| 'days'
	| 'day'
	| 'search'
	| 'instance';

export async function readCachedSnapshot<T>(
	origin: string,
	kind: SnapshotKind,
	id: string
): Promise<T | undefined> {
	const record = await getSnapshot(snapshotKey(origin, kind, id));
	return record?.data as T | undefined;
}

export async function writeCachedSnapshot(
	origin: string,
	kind: SnapshotKind,
	id: string,
	data: unknown
): Promise<void> {
	await putSnapshot(snapshotKey(origin, kind, id), {
		data,
		fetched_at: Date.now()
	});
}

export async function invalidateCircleSnapshots(
	origin: string,
	circleId: string,
	kind?: SnapshotKind
): Promise<void> {
	const opts: InvalidateSnapshotsOptions = { circleId };
	if (kind) opts.kind = kind;
	await invalidateSnapshots(origin, opts);
}

export { invalidateSnapshots };
