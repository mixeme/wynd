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

type BarcodeDetectorLike = {
	detect(image: ImageBitmap): Promise<Array<{ rawValue: string }>>;
};

export async function decodeQrFromFile(file: File): Promise<string> {
	const Detector = (
		globalThis as typeof globalThis & {
			BarcodeDetector?: new (options?: { formats?: string[] }) => BarcodeDetectorLike;
		}
	).BarcodeDetector;
	if (!Detector) {
		throw new Error('no_detector');
	}
	const bitmap = await createImageBitmap(file);
	try {
		const detector = new Detector({ formats: ['qr_code'] });
		const codes = await detector.detect(bitmap);
		if (!codes.length) throw new Error('no_code');
		return codes[0].rawValue;
	} finally {
		bitmap.close();
	}
}
