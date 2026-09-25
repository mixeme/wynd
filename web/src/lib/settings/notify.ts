import { apiJson } from '$lib/api/client';
import { getAppSettings, saveAppSettings } from '$lib/idb/db';
import type { NotifyPrefs } from '$lib/circles/settings';

export async function fetchAccountNotifyPrefs(origin: string): Promise<NotifyPrefs> {
	return apiJson<NotifyPrefs>(origin, '/notify_prefs');
}

export async function saveAccountNotifyPrefs(
	origin: string,
	prefs: Partial<Pick<NotifyPrefs, 'posts' | 'comments' | 'reactions'>>
): Promise<NotifyPrefs> {
	return apiJson<NotifyPrefs>(origin, '/notify_prefs', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(prefs)
	});
}

export async function persistNotifyDefaults(prefs: {
	posts: boolean;
	comments: boolean;
	reactions: boolean;
}): Promise<void> {
	const current = (await getAppSettings()) ?? { theme: 'system' as const };
	await saveAppSettings({
		...current,
		notify_defaults: {
			posts: prefs.posts,
			comments: prefs.comments,
			reactions: prefs.reactions
		}
	});
}
