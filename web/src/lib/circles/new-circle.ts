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
