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
	refresh: () => Promise<void>;
}
