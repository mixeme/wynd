import type { NotifyPrefs } from '$lib/circles/settings';

export type MuteKey = 'none' | 'tomorrow' | 'week';

export function muteKeyFromUntil(until?: string | null): MuteKey {
	if (!until) return 'none';
	const end = Date.parse(until);
	if (Number.isNaN(end) || end <= Date.now()) return 'none';
	const hours = (end - Date.now()) / (3600 * 1000);
	if (hours <= 36) return 'tomorrow';
	return 'week';
}

export function muteUntilFromKey(key: MuteKey): string | null {
	if (key === 'none') return null;
	const now = new Date();
	if (key === 'tomorrow') {
		const d = new Date(now);
		d.setDate(d.getDate() + 1);
		d.setHours(0, 0, 0, 0);
		return d.toISOString();
	}
	const d = new Date(now);
	d.setDate(d.getDate() + 7);
	return d.toISOString();
}

export type NotifyBaseline = Pick<
	NotifyPrefs,
	'posts' | 'comments_mine' | 'comments_all' | 'reactions' | 'events' | 'mute_until'
>;

export function notifyBaselineEqual(a: NotifyBaseline, b: NotifyBaseline): boolean {
	return (
		a.posts === b.posts &&
		a.comments_mine === b.comments_mine &&
		a.comments_all === b.comments_all &&
		a.reactions === b.reactions &&
		a.events === b.events &&
		(a.mute_until ?? null) === (b.mute_until ?? null)
	);
}

export function baselineFromPrefs(prefs: NotifyPrefs): NotifyBaseline {
	return {
		posts: prefs.posts,
		comments_mine: prefs.comments_mine,
		comments_all: prefs.comments_all,
		reactions: prefs.reactions,
		events: prefs.events,
		mute_until: prefs.mute_until ?? null
	};
}
