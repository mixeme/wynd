import { describe, expect, it } from 'vitest';
import { refetchScreens, registerRefetch } from './sync';

// C108: экран, который не дождался первого ответа (холодный запуск офлайн),
// сам себя не перечитает — сеть вернулась, а список кругов оставался пустым.
describe('возврат сети (C108)', () => {
	it('refetchScreens перечитывает все открытые экраны', () => {
		let street = 0;
		let feed = 0;
		const offStreet = registerRefetch({
			origin: 'https://a.example',
			refetch: () => void street++
		});
		const offFeed = registerRefetch({
			origin: 'https://b.example',
			circleId: 'c1',
			refetch: async () => void feed++
		});
		try {
			refetchScreens();
			expect(street).toBe(1);
			expect(feed).toBe(1);
		} finally {
			offStreet();
			offFeed();
		}
	});
});
