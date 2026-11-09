// Приём файлов из Проводника в окне для компьютера (ПК6/ПК6а). В браузере
// моста нет — обработчики не ставятся, поведение не меняется.
import { goto } from '$app/navigation';
import { handComposePhotos } from '$lib/journal/compose-handoff';
import { desktopBridge } from './bridge';
import { circleFromPath } from './drop-path';

let active = $state(false);

export function desktopDropEnabled(): boolean {
	return Boolean(desktopBridge());
}

export function desktopDropActive(): boolean {
	return active;
}

export function beginDesktopDrop(pathname: string): void {
	active = desktopDropEnabled() && circleFromPath(pathname) !== null;
}

export function endDesktopDrop(): void {
	active = false;
}

/** Отпускание: файлы уходят в подготовку новой записи того же круга (4.2). */
export async function acceptDesktopDrop(pathname: string, files: File[]): Promise<boolean> {
	active = false;
	if (!desktopDropEnabled() || files.length === 0) return false;
	const circleId = circleFromPath(pathname);
	if (!circleId) return false;
	handComposePhotos(circleId, files);
	await goto(`/circles/${encodeURIComponent(circleId)}/compose`);
	return true;
}
