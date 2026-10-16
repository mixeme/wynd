import type { CircleColor } from '$lib/theme/colors';
import type { ArchiveCycle, CircleArrival } from './types';
import type { CircleMoveBanner } from '$lib/circles/circles';

export const CIRCLE_CTX = Symbol('circle');

export interface CircleContext {
	origin: string;
	circleId: string;
	name: string;
	color: CircleColor;
	colorHex: string;
	identityName: string;
	identityId: string;
	identityInitial: string;
	avatarBlobId: string;
	avatarUrl: string;
	editWindowSec: number | null | undefined;
	lastReadSeq: number;
	archiveCycle?: ArchiveCycle;
	canWrite: boolean;
	/** Места нет (C42): писать, отвечать, ставить реакции и править нельзя. */
	spaceFull: '' | 'circle' | 'server';
	/** Кто может освободить место (3.10) и не вы ли это. */
	ownerName: string;
	youOwn: boolean;
	/** Круг замер под полный архив с удалением (3.16): писать нельзя никому. */
	frozenAt: string;
	/** Замер под перенос (план 53): переезжает (3.14) или переехал (3.15). */
	move?: CircleMoveBanner;
	/** Приехал с кругом и ещё не входил (1.10): лента сначала показывает встречу. */
	arrival?: CircleArrival;
	/** Имя сервера для 3.11: «Помочь может администратор «…»». */
	serverName: string;
	/** «Место со снимков» (6.1): начальное положение значка места на 4.2. */
	sharePlace: boolean;
	/** «Отклики» (3.17): сколько нового с прошлого просмотра — число на вкладке. */
	responsesUnread: number;
	/** Был ли в круге кто-то ещё. В круге из одного вкладки «Отклики» нет (3.7). */
	hasOthers: boolean;
	refresh: () => Promise<void>;
}
