import { afterEach, expect, it, vi } from 'vitest';
import {
	originalPhotoAccept,
	applyPhotoLocationChoice,
	photoLocationAllowed,
	photoPickerPreference
} from './photos';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { readExif } from '$lib/media/exif';
import { handComposePhotos, takeComposePhotos } from '$lib/journal/compose-handoff';

afterEach(() => {
	delete (window as unknown as { __wyndPhotoLocation?: unknown }).__wyndPhotoLocation;
	vi.useRealTimers();
});

it('сохраняет обычный accept у браузера и прежней оболочки', async () => {
	expect(await originalPhotoAccept('image/*,video/*')).toBe('image/*,video/*');
	expect(await photoPickerPreference()).toBeUndefined();
});

it('использует предпочтение только на Android с галереей без доступа к координатам', async () => {
	let offerChoice = true;
	let stored = '';
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const { id, op, mode } = JSON.parse(message);
			expect(op).toBe('picker');
			if (mode && offerChoice) stored = mode;
			target.onmessage?.({ data: JSON.stringify({ id, protocol: 1, offerChoice, mode: stored }) });
		}
	};
	(window as unknown as { __wyndPhotoLocation: unknown }).__wyndPhotoLocation = target;
	expect(await photoPickerPreference()).toBe('ask');
	expect(await photoPickerPreference('gallery')).toBe('gallery');
	expect(await photoPickerPreference()).toBe('gallery');
	expect(await photoPickerPreference('documents')).toBe('documents');
	offerChoice = false;
	expect(await photoPickerPreference()).toBeUndefined();
	expect(stored).toBe('documents');
});

it('старый мост с неизвестной операцией оставляет штатное поведение', async () => {
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const { id } = JSON.parse(message);
			target.onmessage?.({ data: JSON.stringify({ id, protocol: 1, error: 'busy' }) });
		}
	};
	(window as unknown as { __wyndPhotoLocation: unknown }).__wyndPhotoLocation = target;
	expect(await photoPickerPreference()).toBeUndefined();
});

it('связывает файловый выбор с одноразовым разрешением главной страницы', async () => {
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const { id } = JSON.parse(message);
			queueMicrotask(() =>
				target.onmessage?.({
					data: JSON.stringify({ id, protocol: 1, token: '01234567-89ab-4cde-8123-0123456789ab' })
				})
			);
		}
	};
	(window as unknown as { __wyndPhotoLocation: unknown }).__wyndPhotoLocation = target;
	expect(await originalPhotoAccept('image/*')).toBe(
		'image/*,application/x-wynd-original-01234567-89ab-4cde-8123-0123456789ab'
	);
});

it('не продолжает выбор с повреждённым или просроченным ответом', async () => {
	vi.useFakeTimers();
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const { id } = JSON.parse(message);
			target.onmessage?.({ data: JSON.stringify({ id, protocol: 1, token: '../private' }) });
		}
	};
	(window as unknown as { __wyndPhotoLocation: unknown }).__wyndPhotoLocation = target;
	await expect(originalPhotoAccept('image/*')).rejects.toThrow('Invalid photo selection');
	target.postMessage = () => {};
	const pending = expect(originalPhotoAccept('image/*')).rejects.toThrow('timed out');
	await vi.advanceTimersByTimeAsync(5000);
	await pending;
});

it('при отказе оставляет дату, но не подставляет GPS; другое File не затрагивается', async () => {
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const { id } = JSON.parse(message);
			queueMicrotask(() =>
				target.onmessage?.({ data: JSON.stringify({ id, protocol: 1, allowed: false }) })
			);
		}
	};
	(window as unknown as { __wyndPhotoLocation: unknown }).__wyndPhotoLocation = target;
	const bytes = new Uint8Array(
		readFileSync(resolve(process.cwd(), 'src/test/fixtures/exif-gps.jpg'))
	);
	const denied = new File([bytes], 'photo.jpg', { type: 'image/jpeg' });
	const other = new File([bytes], 'photo.jpg', { type: 'image/jpeg' });
	await applyPhotoLocationChoice(
		[denied],
		'image/*,application/x-wynd-original-01234567-89ab-4cde-8123-0123456789ab'
	);
	expect(photoLocationAllowed(denied)).toBe(false);
	expect(photoLocationAllowed(other)).toBe(true);
	handComposePhotos('native-photo-test', [denied]);
	const [handed] = takeComposePhotos('native-photo-test');
	expect(handed).toBe(denied);
	const hints = await readExif(handed);
	expect(hints.entry_date).toBe('2026-08-14');
	expect(hints.geo_lat).toBeUndefined();
	expect(hints.geo_lng).toBeUndefined();
	expect((await readExif(other)).geo_lng).toBeCloseTo(30.309, 3);
});

it('совместим с ранней оболочкой без протокола результата', async () => {
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const { id } = JSON.parse(message);
			queueMicrotask(() =>
				target.onmessage?.({
					data: JSON.stringify({ id, token: '01234567-89ab-4cde-8123-0123456789ab' })
				})
			);
		}
	};
	(window as unknown as { __wyndPhotoLocation: unknown }).__wyndPhotoLocation = target;
	expect(await originalPhotoAccept('image/*')).toBe('image/*');
});
