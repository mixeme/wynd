import { mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import DayCard from './DayCard.svelte';

describe('DayCard', () => {
	it('draws a video element when the cover file is a video', () => {
		const target = document.createElement('div');
		document.body.appendChild(target);
		const app = mount(DayCard, {
			target,
			props: {
				title: '4 октября',
				subtitle: '1 запись',
				coverUrl: 'blob:video',
				kind: 'video'
			}
		});
		expect(target.querySelector('video')?.getAttribute('src')).toBe('blob:video');
		expect(target.querySelector('img')).toBeNull();
		unmount(app);
		target.remove();
	});
});
