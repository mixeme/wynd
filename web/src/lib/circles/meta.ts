import { getAppSettings, saveAppSettings } from '$lib/idb/db';
import { normalizeOrigin } from '$lib/api/client';
import type { CircleColor } from '$lib/theme/colors';

export function circleMetaKey(origin: string, circleId: string): string {
	return `${normalizeOrigin(origin)}:${circleId}`;
}

export async function getCircleColor(
	origin: string,
	circleId: string
): Promise<CircleColor | undefined> {
	const settings = await getAppSettings();
	return settings?.circle_meta?.[circleMetaKey(origin, circleId)]?.color;
}

export async function setCircleColor(
	origin: string,
	circleId: string,
	color: CircleColor
): Promise<void> {
	const settings = (await getAppSettings()) ?? { theme: 'system' as const };
	const key = circleMetaKey(origin, circleId);
	const prev = settings.circle_meta?.[key] ?? {};
	const circle_meta = { ...settings.circle_meta, [key]: { ...prev, color } };
	await saveAppSettings({ ...settings, circle_meta });
}

export function circleInitial(name: string): string {
	const trimmed = name.trim();
	return trimmed ? trimmed[0].toUpperCase() : '?';
}

export async function getCircleIdentity(
	origin: string,
	circleId: string
): Promise<string | undefined> {
	const settings = await getAppSettings();
	return settings?.circle_meta?.[circleMetaKey(origin, circleId)]?.identity_name;
}

export async function setCircleIdentity(
	origin: string,
	circleId: string,
	identityName: string
): Promise<void> {
	const settings = (await getAppSettings()) ?? { theme: 'system' as const };
	const key = circleMetaKey(origin, circleId);
	const prev = settings.circle_meta?.[key] ?? {};
	const circle_meta = { ...settings.circle_meta, [key]: { ...prev, identity_name: identityName } };
	await saveAppSettings({ ...settings, circle_meta });
}
