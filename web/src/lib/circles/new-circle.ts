import type { CircleColor } from '$lib/circles/circles';
import type { SessionRecord } from '$lib/idb/db';

export const NEW_CIRCLE_CTX = Symbol('newCircle');

export type NewCircleEditWindow = 'chronicle' | '10m' | '1h' | '1d' | 'unlimited' | 'custom';

export interface NewCircleContext {
	sessions: SessionRecord[];
	circleCounts: Record<string, number>;
	selectedOrigin: string;
	name: string;
	color: CircleColor;
	editWindow: NewCircleEditWindow;
	customHours: number;
	diaryMode: boolean;
	loading: boolean;
	error: string;
	ready: boolean;
	serverSubtitle: (session: SessionRecord) => string;
}

/** Окно правок формы в секундах; null — без ограничения. */
export function newCircleEditWindowSec(form: Pick<NewCircleContext, 'editWindow' | 'customHours'>): number | null {
	switch (form.editWindow) {
		case 'chronicle':
			return 0;
		case '10m':
			return 600;
		case '1h':
			return 3600;
		case '1d':
			return 86400;
		case 'custom':
			return Math.max(1, Math.min(8760, form.customHours)) * 3600;
		default:
			return null;
	}
}
