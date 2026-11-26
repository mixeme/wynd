import { describe, expect, it } from 'vitest';
import {
	ANDROID_RELEASES_PAGE,
	fetchLatestAndroidAppRelease,
	pickAndroidAppRelease
} from './appRelease';

function release(
	tag: string,
	patch: Partial<{
		draft: boolean;
		prerelease: boolean;
		assets: { name: string; browser_download_url: string }[];
	}> = {}
) {
	return {
		tag_name: tag,
		draft: patch.draft ?? false,
		prerelease: patch.prerelease ?? false,
		html_url: `https://github.com/mixeme/wynd/releases/tag/${tag}`,
		assets: patch.assets ?? [
			{
				name: 'wynd-android.apk',
				browser_download_url: `https://github.com/mixeme/wynd/releases/download/${tag}/wynd-android.apk`
			},
			{
				name: 'wynd-android-unsigned.apk',
				browser_download_url: `https://github.com/mixeme/wynd/releases/download/${tag}/wynd-android-unsigned.apk`
			}
		]
	};
}

describe('последний выпуск приложения Android (C139)', () => {
	it('берёт самый новый по номеру, а не по порядку списка', () => {
		const picked = pickAndroidAppRelease([
			release('android-v0.1.3'),
			release('android-v0.1.5'),
			release('android-v0.1.4')
		]);
		expect(picked?.version).toBe('0.1.5');
		expect(picked?.apkUrl).toContain('/android-v0.1.5/wynd-android.apk');
		expect(picked?.pageUrl).toContain('/android-v0.1.5');
	});

	it('пропускает черновики, предварительные и чужие выпуски', () => {
		const picked = pickAndroidAppRelease([
			release('android-v0.1.6', { draft: true }),
			release('android-v0.1.7', { prerelease: true }),
			release('desktop-v0.5.2'),
			release('workshop-v0.4.10'),
			release('android-v0.1.5')
		]);
		expect(picked?.version).toBe('0.1.5');
	});

	it('выпуск без подписанного APK не годится', () => {
		const picked = pickAndroidAppRelease([
			release('android-v0.1.6', {
				assets: [
					{
						name: 'wynd-android-unsigned.apk',
						browser_download_url: 'https://github.com/mixeme/wynd/releases/download/x/u.apk'
					}
				]
			}),
			release('android-v0.1.5')
		]);
		expect(picked?.version).toBe('0.1.5');
	});

	it('без выпусков Android ничего не выбирает', () => {
		expect(pickAndroidAppRelease([release('desktop-v0.5.2')])).toBeUndefined();
	});

	it('при отказе сети остаётся страница выпусков', async () => {
		expect(await fetchLatestAndroidAppRelease(async () => new Response('', { status: 500 }))).toBe(
			undefined
		);
		expect(
			await fetchLatestAndroidAppRelease(async () => {
				throw new Error('offline');
			})
		).toBeUndefined();
		expect(
			await fetchLatestAndroidAppRelease(
				async () => ({ ok: true, json: async () => [release('android-v0.1.5')] }) as Response
			)
		).toEqual({
			version: '0.1.5',
			apkUrl: 'https://github.com/mixeme/wynd/releases/download/android-v0.1.5/wynd-android.apk',
			pageUrl: 'https://github.com/mixeme/wynd/releases/tag/android-v0.1.5'
		});
		expect(ANDROID_RELEASES_PAGE).toContain('github.com/mixeme/wynd/releases');
	});
});
