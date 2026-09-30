import type { CircleColor } from '$lib/theme/colors';
import type { ArchiveCycle } from './types';

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
	/** «Место со снимков» (6.1): начальное положение значка места на 4.2. */
	sharePlace: boolean;
	/** «Отклики» (3.12): сколько нового с прошлого просмотра — число на вкладке. */
	responsesUnread: number;
	/** Был ли в круге кто-то ещё. В круге из одного вкладки «Отклики» нет (3.7). */
	hasOthers: boolean;
	refresh: () => Promise<void>;
}
