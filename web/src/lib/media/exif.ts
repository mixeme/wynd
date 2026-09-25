import exifr from 'exifr';

export interface ExifHints {
	captured_at?: string;
	entry_date?: string;
	geo_lat?: number;
	geo_lng?: number;
}

export async function readExif(file: File): Promise<ExifHints> {
	try {
		// `latitude`/`longitude` are computed by exifr, not tags: `pick` would drop them,
		// so select the exif and gps segments instead.
		const data = await exifr.parse(file, { exif: true, gps: true });
		if (!data) return {};
		const hints: ExifHints = {};
		if (data.DateTimeOriginal instanceof Date) {
			hints.captured_at = data.DateTimeOriginal.toISOString();
			const y = data.DateTimeOriginal.getFullYear();
			const m = String(data.DateTimeOriginal.getMonth() + 1).padStart(2, '0');
			const d = String(data.DateTimeOriginal.getDate()).padStart(2, '0');
			hints.entry_date = `${y}-${m}-${d}`;
		}
		if (typeof data.latitude === 'number') hints.geo_lat = data.latitude;
		if (typeof data.longitude === 'number') hints.geo_lng = data.longitude;
		return hints;
	} catch {
		return {};
	}
}
