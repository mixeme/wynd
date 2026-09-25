import { getMediaUrl } from '$lib/media/objectUrl';

/** Сколько адресов медиа тянем разом. */
export const MEDIA_BATCH = 8;

/**
 * Берёт адреса блобов пачками и отдаёт каждую сразу, как она готова.
 *
 * Экраны делали это последовательным `await` в цикле: лента ждала до двух
 * тысяч ответов подряд, альбом — по одному на снимок (REF-6). Неудачный
 * блоб пропускается: одна битая картинка не должна ронять остальные.
 */
export async function resolveMediaUrls(
	origin: string,
	blobIds: string[],
	apply: (blobId: string, url: string) => void,
	batchSize: number = MEDIA_BATCH
): Promise<void> {
	for (let i = 0; i < blobIds.length; i += batchSize) {
		const batch = blobIds.slice(i, i + batchSize);
		const urls = await Promise.all(
			batch.map((blobId) => getMediaUrl(origin, blobId).catch(() => ''))
		);
		batch.forEach((blobId, idx) => {
			if (urls[idx]) apply(blobId, urls[idx]);
		});
	}
}
