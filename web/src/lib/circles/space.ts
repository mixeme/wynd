import { formatSessionDay } from '$lib/format/time';

/**
 * Места нет (план 46, C42; кадры 3.10, 3.11): квота считает всё, что круг
 * кладёт на диск, включая текст, и при упоре писать в круг нельзя совсем.
 * Полоса на месте строки ввода говорит, что случилось и кто может помочь.
 */
export interface SpaceNotice {
	first: string;
	/** Вторая строка текстом — или ссылкой у владельца круга. */
	second: string;
	href?: string;
}

export function spaceNotice(ctx: {
	spaceFull: '' | 'circle' | 'server';
	ownerName: string;
	youOwn: boolean;
	serverName: string;
	circleId: string;
	/** Идёт архивация (6.21): место освободится в её срок. */
	archiveCycle?: { active: boolean; deadline: string };
}): SpaceNotice | undefined {
	if (ctx.spaceFull === 'server') {
		// Владелец круга тут бессилен — у него полоса та же, без ссылки (3.11).
		const server = ctx.serverName ? ` «${ctx.serverName}»` : ' сервера';
		return {
			first: 'На сервере кончилось место — писать пока нельзя.',
			second: `Помочь может администратор${server}.`
		};
	}
	if (ctx.spaceFull !== 'circle') return undefined;
	const first = 'В круге кончилось место — писать пока нельзя.';
	// Архивация уже идёт — место освободится само, к сроку; звать освобождать
	// незачем (владелец, 2026-10-15).
	if (ctx.archiveCycle?.active && ctx.archiveCycle.deadline) {
		return { first, second: `Место освободится ${formatSessionDay(ctx.archiveCycle.deadline)}` };
	}
	if (ctx.youOwn) {
		return {
			first,
			second: 'Освободить место или попросить больше места',
			href: `/circles/${ctx.circleId}/quota`
		};
	}
	const who = ctx.ownerName || 'владелец круга';
	return { first, second: `Освободить или попросить больше места может ${who}.` };
}
