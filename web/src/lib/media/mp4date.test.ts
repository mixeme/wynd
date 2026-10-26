import { describe, expect, it } from 'vitest';
import { mp4CreationTime, readVideoDate } from './mp4date';

const EPOCH_1904 = Date.UTC(1904, 0, 1);

function box(type: string, body: Uint8Array): Uint8Array {
	const out = new Uint8Array(8 + body.length);
	new DataView(out.buffer).setUint32(0, out.length);
	for (let i = 0; i < 4; i++) out[4 + i] = type.charCodeAt(i);
	out.set(body, 8);
	return out;
}

function mvhd(at: Date, version: 0 | 1): Uint8Array {
	const body = new Uint8Array(version === 1 ? 32 : 24);
	const v = new DataView(body.buffer);
	v.setUint8(0, version);
	const seconds = Math.floor((at.getTime() - EPOCH_1904) / 1000);
	if (version === 1) v.setBigUint64(4, BigInt(seconds));
	else v.setUint32(4, seconds);
	return box('mvhd', body);
}

function file(...parts: Uint8Array[]): Blob {
	return new Blob(parts as BlobPart[]);
}

const shot = new Date(Date.UTC(2026, 8, 28, 10, 30, 0));

describe('дата съёмки ролика из заголовка MP4 (A14)', () => {
	it('moov в начале, mvhd первой версии 0', async () => {
		const f = file(
			box('ftyp', new Uint8Array(8)),
			box('moov', mvhd(shot, 0)),
			box('mdat', new Uint8Array(100))
		);
		expect((await mp4CreationTime(f))?.toISOString()).toBe(shot.toISOString());
	});

	it('moov после данных ролика и mvhd версии 1', async () => {
		const f = file(
			box('ftyp', new Uint8Array(8)),
			box('mdat', new Uint8Array(5000)),
			box('moov', mvhd(shot, 1))
		);
		expect((await mp4CreationTime(f))?.toISOString()).toBe(shot.toISOString());
	});

	it('день записи — по местному времени, как у снимка', async () => {
		const f = file(box('moov', mvhd(shot, 0)));
		const hints = await readVideoDate(f, new Date(Date.UTC(2026, 9, 23)));
		const local = new Date(shot);
		const want = `${local.getFullYear()}-${String(local.getMonth() + 1).padStart(2, '0')}-${String(local.getDate()).padStart(2, '0')}`;
		expect(hints).toEqual({ captured_at: shot.toISOString(), entry_date: want });
	});

	it('нет даты, ноль или не MP4 — пусто', async () => {
		expect(await readVideoDate(file(box('moov', mvhd(new Date(EPOCH_1904), 0))))).toEqual({});
		expect(await readVideoDate(file(new TextEncoder().encode('\x1aE\xdf\xa3 webm')))).toEqual({});
		expect(await readVideoDate(file(new Uint8Array(3)))).toEqual({});
	});

	it('дату из будущего не берёт', async () => {
		const later = new Date(Date.UTC(2030, 0, 1));
		expect(
			await readVideoDate(file(box('moov', mvhd(later, 0))), new Date(Date.UTC(2026, 9, 23)))
		).toEqual({});
	});
});
