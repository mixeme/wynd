/** Path for in-app navigation, or null if the text is not a Wynd invite/join link. */
export function parseWyndLink(input: string): string | null {
	const raw = input.trim();
	if (!raw) return null;
	try {
		const base = typeof window !== 'undefined' ? window.location.origin : 'https://local';
		const url = new URL(raw.includes('://') || raw.startsWith('/') ? raw : `https://${raw}`, base);
		const invite = url.pathname.match(/^\/invite\/([^/?#]+)/);
		if (invite) return `/invite/${invite[1]}${url.search}`;
		const join = url.pathname.match(/^\/join\/([^/?#]+)/);
		if (join) return `/join/${join[1]}${url.search}`;
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
