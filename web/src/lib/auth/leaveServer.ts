import { apiJson, ApiError } from '$lib/api/client';
import { WORD, plural, pluralForm } from '$lib/format/plural';

/**
 * Уход с сервера насовсем (кадры 7.13–7.17, план 46, A3). Сервер выбирают в
 * «Серверах» — до экрана ухода это живёт во вкладке, как у смены почты.
 */
export interface PendingLeave {
	origin: string;
	serverName: string;
	host: string;
	email: string;
}

const KEY = 'wynd:pending-leave';

export function loadPendingLeave(): PendingLeave | undefined {
	if (typeof sessionStorage === 'undefined') return undefined;
	try {
		const raw = sessionStorage.getItem(KEY);
		if (!raw) return undefined;
		return JSON.parse(raw) as PendingLeave;
	} catch {
		return undefined;
	}
}

export function savePendingLeave(pending: PendingLeave): void {
	sessionStorage.setItem(KEY, JSON.stringify(pending));
}

export function clearPendingLeave(): void {
	sessionStorage.removeItem(KEY);
}

export interface OwnedCircle {
	id: string;
	name: string;
	/** Действующих участников вместе с владельцем. */
	members: number;
}

export interface LeavePreview {
	circles: number;
	owned: OwnedCircle[];
}

export async function fetchLeavePreview(origin: string): Promise<LeavePreview> {
	return apiJson<LeavePreview>(origin, '/auth/leave');
}

export async function leaveServer(origin: string, eraseNames: boolean): Promise<void> {
	await apiJson(origin, '/auth/leave', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ erase_names: eraseNames })
	});
}

/** Ответ сервера на уход — словами экрана. */
export function leaveErrorHint(err: unknown): string {
	if (err instanceof ApiError) {
		if (err.status === 409) return 'Вы ещё владеете кругами на этом сервере — сначала передайте или удалите их.';
		if (err.status === 403) return 'Стирать имена и фото этот сервер больше не разрешает.';
	}
	return 'Не получилось уйти — проверьте связь и попробуйте ещё раз.';
}

const OWNED_WORDS = ['', 'одного', 'двух', 'трёх', 'четырёх'];

/** «Вы владелец двух кругов» (7.15): «владелец» правит родительным падежом. */
export function ownedCirclesLine(n: number): string {
	if (n === 1) return 'Вы владелец круга.';
	return `Вы владелец ${OWNED_WORDS[n] ?? n} ${pluralForm(n, WORD.circleGenitive)}.`;
}

/** Подпись круга в списке 7.15: куда ведёт строка. */
export function ownedCircleSubtitle(c: OwnedCircle): string {
	if (c.members <= 1) return 'кроме вас никого · удалить круг';
	return `${plural(c.members, WORD.member)} · передать владение`;
}

/** Окно подтверждения (7.16, 7.17): что будет, сказано до кнопки. */
export function leaveConfirmText(circles: number, eraseNames: boolean): string {
	if (!eraseNames) {
		return (
			'Вы выйдете со всех устройств. Вернуться с этой почтой можно, но как новый человек: ' +
			'в круги — по новому приглашению, и прежнее в них будет не видно. ' +
			'Ваши записи останутся под вашими именами.'
		);
	}
	const where =
		circles === 1
			? 'В круге'
			: circles > 1
				? `Во всех ${circles} ${pluralForm(circles, WORD.circlePrepositional)}`
				: 'Во всех кругах';
	return (
		`Вы выйдете со всех устройств. ${where} сотрутся ваши имена и фото, которыми подписаны ` +
		'ваши записи, — записи останутся без имени. Вернуть подпись будет нельзя. ' +
		'Вернуться с этой почтой можно, но как новый человек: в круги — по новому приглашению.'
	);
}
