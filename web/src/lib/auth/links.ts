/**
 * Path for in-app navigation, or null if the text is not a Wynd invite/join
 * link. Ссылка на другой сервер не принимается: клиент отправил бы её токен
 * на текущий сервер, где он попал бы в логи (аудит 2026-09-22). Без схемы и
 * хоста (`/invite/…`) считается своей.
 */
export function parseWyndLink(input: string, currentOrigin?: string): string | null {
	const raw = input.trim();
	if (!raw) return null;
	try {
		const base = currentOrigin ?? (typeof window !== 'undefined' ? window.location.origin : 'https://local');
		const url = new URL(raw.includes('://') || raw.startsWith('/') ? raw : `https://${raw}`, base);
		if (url.origin !== new URL(base).origin) return null;
		const invite = url.pathname.match(/^\/invite\/([^/?#]+)/);
		if (invite) return `/invite/${invite[1]}${url.search}`;
		const join = url.pathname.match(/^\/join\/([^/?#]+)/);
		if (join) return `/join/${join[1]}${url.search}`;
	} catch {
		/* not a URL */
	}
	return null;
}

/** Origin of a Wynd invite/join link that points at another server, else null. */
export function foreignWyndLinkOrigin(input: string, currentOrigin?: string): string | null {
	const raw = input.trim();
	if (!raw.includes('://')) return null;
	try {
		const base = currentOrigin ?? (typeof window !== 'undefined' ? window.location.origin : 'https://local');
		const url = new URL(raw);
		if (url.origin === new URL(base).origin) return null;
		if (/^\/(invite|join)\/[^/?#]+/.test(url.pathname)) return url.origin;
	} catch {
		/* not a URL */
	}
	return null;
}

/**
 * Куда ведёт присланный текст (вставка, QR): путь приглашения или отказ
 * словами. Ссылка на другой сервер не открывается здесь — её токен ушёл бы
 * на этот сервер.
 */
export function inviteTarget(
	text: string,
	currentOrigin?: string
): { path: string } | { error: string } {
	const foreign = foreignWyndLinkOrigin(text, currentOrigin);
	if (foreign) return { error: `Ссылка ведёт на другой сервер (${foreign}) — откройте её там` };
	const path = parseWyndLink(text, currentOrigin);
	if (!path) return { error: 'Это не ссылка-приглашение Wynd' };
	return { path };
}

type BarcodeDetectorLike = {
	detect(image: ImageBitmapSource): Promise<Array<{ rawValue: string }>>;
};

function nativeDetector(): BarcodeDetectorLike | null {
	const Detector = (
		globalThis as typeof globalThis & {
			BarcodeDetector?: new (options?: { formats?: string[] }) => BarcodeDetectorLike;
		}
	).BarcodeDetector;
	return Detector ? new Detector({ formats: ['qr_code'] }) : null;
}

const scratch = typeof document !== 'undefined' ? document.createElement('canvas') : null;

/**
 * Код с кадра или картинки. Встроенный BarcodeDetector есть в Chrome на
 * Android, в Firefox его нет — там распознаёт jsQR (подгружается, только
 * когда нужен). Без кода — null.
 */
export async function decodeQrFrom(
	source: HTMLVideoElement | ImageBitmap,
	width: number,
	height: number
): Promise<string | null> {
	const detector = nativeDetector();
	if (detector) {
		try {
			const codes = await detector.detect(source);
			return codes[0]?.rawValue ?? null;
		} catch {
			/* нет формата qr_code в этой сборке — ниже jsQR */
		}
	}
	if (!scratch || !width || !height) return null;
	// Кадр уменьшается до 640 по большей стороне: jsQR на 4K-кадре медленный.
	const scale = Math.min(1, 640 / Math.max(width, height));
	scratch.width = Math.round(width * scale);
	scratch.height = Math.round(height * scale);
	const ctx = scratch.getContext('2d', { willReadFrequently: true });
	if (!ctx) return null;
	ctx.drawImage(source, 0, 0, scratch.width, scratch.height);
	const { default: jsQR } = await import('jsqr');
	const image = ctx.getImageData(0, 0, scratch.width, scratch.height);
	return jsQR(image.data, image.width, image.height)?.data ?? null;
}

export async function decodeQrFromFile(file: File): Promise<string> {
	const bitmap = await createImageBitmap(file);
	try {
		const text = await decodeQrFrom(bitmap, bitmap.width, bitmap.height);
		if (!text) throw new Error('no_code');
		return text;
	} finally {
		bitmap.close();
	}
}
