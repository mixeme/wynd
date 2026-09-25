import { apiJson } from '$lib/api/client';
import { getAppSettings, saveAppSettings } from '$lib/idb/db';
import type { NotifyPrefs } from '$lib/circles/settings';

export type AppNotifyDefaults = {
	posts: boolean;
	comments_mine: boolean;
	reactions: boolean;
};

export function appNotifyDefaultsEqual(a: AppNotifyDefaults, b: AppNotifyDefaults): boolean {
	return a.posts === b.posts && a.comments_mine === b.comments_mine && a.reactions === b.reactions;
}

export function accountNotifyPrefsFromAppDefaults(prefs: AppNotifyDefaults) {
	return {
		posts: prefs.posts,
		comments_mine: prefs.comments_mine,
		reactions: prefs.reactions,
		comments_all: false,
		events: false,
		mute_until: null as string | null
	};
}

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

export async function persistNotifyDefaults(prefs: AppNotifyDefaults): Promise<void> {
	const current = (await getAppSettings()) ?? { theme: 'system' as const };
	await saveAppSettings({
		...current,
		notify_defaults: {
			posts: prefs.posts,
			comments_mine: prefs.comments_mine,
			reactions: prefs.reactions
		}
	});
}
