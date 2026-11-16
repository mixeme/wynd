import { afterEach, expect, it, vi } from 'vitest';
import { postAndroidAppearance } from './appearance';

afterEach(() => {
	delete (window as unknown as { __wyndAppearance?: unknown }).__wyndAppearance;
});

it('работает без дополнительной возможности оболочки', () => {
	expect(() => postAndroidAppearance('#F4F0E9', '#211E1C')).not.toThrow();
});

it('передаёт только пару непрозрачных RGB без иных команд', () => {
	const postMessage = vi.fn();
	(window as unknown as { __wyndAppearance: unknown }).__wyndAppearance = { postMessage };
	postAndroidAppearance('#F4F0E9', '#211E1C');
	expect(JSON.parse(postMessage.mock.calls[0][0])).toEqual({
		status: '#F4F0E9',
		navigation: '#211E1C'
	});
	for (const color of ['transparent', '#fff', '#12345678', 'red', '#FFFFFF\n'])
		postAndroidAppearance(color, '#211E1C');
	expect(postMessage).toHaveBeenCalledTimes(1);
});
