import { apiJson } from '$lib/api/client';
import { getAppSettings, saveAppSettings } from '$lib/idb/db';
import type { NotifyPrefs } from '$lib/circles/settings';

export async function fetchAccountNotifyPrefs(origin: string): Promise<NotifyPrefs> {
	return apiJson<NotifyPrefs>(origin, '/notify_prefs');
}

export async function saveAccountNotifyPrefs(
	origin: string,
	prefs: Partial<
		Pick<
			NotifyPrefs,
			'posts' | 'comments_mine' | 'comments_all' | 'reactions' | 'events' | 'mute_until'
		>
	>
): Promise<NotifyPrefs> {
	return apiJson<NotifyPrefs>(origin, '/notify_prefs', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(prefs)
	});
}

export async function persistNotifyDefaults(prefs: {
	posts: boolean;
	comments_mine: boolean;
	comments_all: boolean;
	reactions: boolean;
	events: boolean;
	mute_until?: string | null;
}): Promise<void> {
	const current = (await getAppSettings()) ?? { theme: 'system' as const };
	await saveAppSettings({
		...current,
		notify_defaults: {
			posts: prefs.posts,
			comments_mine: prefs.comments_mine,
			comments_all: prefs.comments_all,
			reactions: prefs.reactions,
			events: prefs.events,
			mute_until: prefs.mute_until ?? null
		}
	});
}
