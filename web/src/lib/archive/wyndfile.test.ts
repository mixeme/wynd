import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
	WrongPasswordError,
	blobSource,
	decryptedSource,
	inspectWynd,
	openArchive,
	type RandomSource,
	type WyndTable
} from './wyndfile';

// Файлы собраны сервером (internal/archive, 0.31.0): круг «Семья», одна
// запись и снимок 150 000 байт — три куска потока age.
function fixture(name: string): RandomSource {
	const buf = readFileSync(resolve(process.cwd(), `src/test/fixtures/full-archive-${name}`));
	return blobSource(new Blob([buf]));
}

const PHOTO = 'media/00000000-0000-7000-8000-0000000000aa.jpg';

async function checkArchive(src: RandomSource) {
	const { zip, manifest } = await openArchive(src);
	expect(manifest.circle_name).toBe('Семья');
	expect(manifest.schema_version).toBe(34);
	const posts = await zip.json<WyndTable>('data/posts.json');
	expect(posts.columns).toEqual(['id', 'body']);
	expect(String(posts.rows[0][1])).toContain('на даче');
	let total = 0;
	let ok = true;
	for await (const part of zip.chunks(PHOTO, 40_000)) {
		for (let i = 0; i < part.length; i++) {
			const at = total + i;
			if (part[i] !== ((at * 7 + Math.floor(at / 251)) & 0xff)) ok = false;
		}
		total += part.length;
	}
	expect(total).toBe(150_000);
	expect(ok).toBe(true);
}

describe('файл полного архива', () => {
	it('открытый читается по оглавлению', async () => {
		const src = fixture('plain.wynd');
		expect((await inspectWynd(src)).encrypted).toBe(false);
		await checkArchive(src);
	});

	it('зашифрованный — паролем, по кускам', async () => {
		const src = fixture('encrypted.wynd');
		const info = await inspectWynd(src);
		if (!info.encrypted) throw new Error('ждали зашифрованный');
		expect(info.header.log_n).toBe(15);
		const plain = await decryptedSource(src, info.header, info.headerLength, 'дача-самовар-облако-17');
		expect(plain.size).toBe(fixture('plain.wynd').size);
		await checkArchive(plain);
	}, 30_000);

	it('неверный пароль', async () => {
		const src = fixture('encrypted.wynd');
		const info = await inspectWynd(src);
		if (!info.encrypted) throw new Error('ждали зашифрованный');
		await expect(
			decryptedSource(src, info.header, info.headerLength, 'дача-самовар-облако-71')
		).rejects.toBeInstanceOf(WrongPasswordError);
	}, 30_000);
});
