import { dev } from '$app/environment';
import { error } from '@sveltejs/kit';

// Smoke, spike and UI-library pages are development aids. They ship in the
// bundle (SvelteKit cannot drop routes per build), so refuse them at runtime
// outside `vite dev`.
export function load(): void {
	if (!dev) error(404, 'Not found');
}
