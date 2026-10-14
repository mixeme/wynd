import { chacha20poly1305 } from '@noble/ciphers/chacha.js';
import { x25519 } from '@noble/curves/ed25519.js';
import { hkdf } from '@noble/hashes/hkdf.js';
import { hmac } from '@noble/hashes/hmac.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { derivePrivateKey } from './fullkey';

/**
 * Чтение файла полного архива `.wynd` на устройстве (план 52). Файл бывает в
 * гигабайты, поэтому читается выборочно: оглавление ZIP с конца, потом нужные
 * записи. Зашифрованный — так же: формат age позволяет расшифровать любой
 * кусок потока по его номеру, и устройство расшифровывает только то, что
 * читает. Пароль не уходит с устройства.
 */

/** Источник с произвольным доступом: файл на диске или его расшифровка. */
export interface RandomSource {
	size: number;
	read(offset: number, length: number): Promise<Uint8Array>;
}

export function blobSource(blob: Blob): RandomSource {
	return {
		size: blob.size,
		async read(offset, length) {
			return new Uint8Array(await blob.slice(offset, offset + length).arrayBuffer());
		}
	};
}

export const ENCRYPTED_MAGIC = 'wynd-encrypted/v1';

export interface EncryptedHeader {
	kdf: string;
	log_n: number;
	r: number;
	p: number;
	salt: string;
	recipient: string;
}

export class WrongPasswordError extends Error {
	constructor() {
		super('wrong password');
	}
}

export class NotWyndError extends Error {
	constructor(why: string) {
		super(`not a wynd archive: ${why}`);
	}
}

const utf8 = new TextDecoder();
const enc = new TextEncoder();

function fromBase64(s: string): Uint8Array {
	const bin = atob(s.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (s.length % 4)) % 4));
	const out = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
	return out;
}

function concat(...parts: Uint8Array[]): Uint8Array {
	const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
	let off = 0;
	for (const p of parts) {
		out.set(p, off);
		off += p.length;
	}
	return out;
}

/** Зашифрован ли файл — и параметры ключа, если да. */
export async function inspectWynd(
	src: RandomSource
): Promise<{ encrypted: false } | { encrypted: true; header: EncryptedHeader; headerLength: number }> {
	const head = await src.read(0, Math.min(src.size, 4096));
	if (head[0] === 0x50 && head[1] === 0x4b) return { encrypted: false };
	const text = utf8.decode(head);
	const first = text.indexOf('\n');
	if (first < 0 || text.slice(0, first) !== ENCRYPTED_MAGIC) throw new NotWyndError('no magic');
	const second = text.indexOf('\n', first + 1);
	if (second < 0) throw new NotWyndError('short header');
	const header = JSON.parse(text.slice(first + 1, second)) as EncryptedHeader;
	if (header.kdf !== 'scrypt' || header.r !== 8 || header.p !== 1) throw new NotWyndError('kdf');
	// Длина в байтах, а не в символах: JSON — ASCII, но магия тоже.
	const headerLength = enc.encode(text.slice(0, second + 1)).length;
	return { encrypted: true, header, headerLength };
}

const AGE_INTRO = 'age-encryption.org/v1\n';
const CHUNK = 64 * 1024;
const TAG = 16;

/**
 * Расшифровка age (X25519) с произвольным доступом: заголовок age, ключ
 * файла, ключ потока, затем кусок i — ChaCha20-Poly1305 с nonce «номер куска
 * (11 байт) + признак последнего». Неверный пароль — WrongPasswordError.
 */
export async function decryptedSource(
	src: RandomSource,
	header: EncryptedHeader,
	headerLength: number,
	password: string,
	onKeyProgress?: (f: number) => void
): Promise<RandomSource> {
	const salt = fromBase64(header.salt);
	const priv = await derivePrivateKey(password, salt, header.log_n, onKeyProgress);
	const pub = x25519.getPublicKey(priv);

	// Заголовок age: до «--- MAC\n». Он короткий — хватает первых 4 КБ.
	const ageHead = await src.read(headerLength, Math.min(4096, src.size - headerLength));
	const text = utf8.decode(ageHead);
	if (!text.startsWith(AGE_INTRO)) throw new NotWyndError('age intro');
	const macAt = text.indexOf('\n---');
	if (macAt < 0) throw new NotWyndError('age header');
	const macLineEnd = text.indexOf('\n', macAt + 1);
	const lines = text.slice(AGE_INTRO.length, macAt).split('\n');
	let fileKey: Uint8Array | undefined;
	for (let i = 0; i < lines.length; i++) {
		const args = lines[i].split(' ');
		if (args[0] !== '->') continue;
		let body = '';
		for (let j = i + 1; j < lines.length && !lines[j].startsWith('->'); j++) {
			body += lines[j];
			if (lines[j].length < 64) break;
		}
		if (args[1] !== 'X25519') continue;
		const share = fromBase64(args[2]);
		const shared = x25519.getSharedSecret(priv, share);
		const wrapKey = hkdf(sha256, shared, concat(share, pub), enc.encode('age-encryption.org/v1/X25519'), 32);
		try {
			fileKey = chacha20poly1305(wrapKey, new Uint8Array(12)).decrypt(fromBase64(body));
			break;
		} catch {
			/* не этот получатель */
		}
	}
	priv.fill(0);
	if (!fileKey) throw new WrongPasswordError();

	// MAC заголовка: от начала до «---» включительно.
	const macKey = hkdf(sha256, fileKey, new Uint8Array(0), enc.encode('header'), 32);
	const headerBytes = enc.encode(text.slice(0, macAt + 4));
	const want = fromBase64(text.slice(macAt + 5, macLineEnd));
	const got = hmac(sha256, macKey, headerBytes);
	if (want.length !== got.length || want.some((b, i) => b !== got[i])) {
		throw new NotWyndError('header mac');
	}

	const ageHeaderLength = enc.encode(text.slice(0, macLineEnd + 1)).length;
	const payloadStart = headerLength + ageHeaderLength + 16;
	const nonce = await src.read(headerLength + ageHeaderLength, 16);
	const payloadKey = hkdf(sha256, fileKey, nonce, enc.encode('payload'), 32);
	const cipherLength = src.size - payloadStart;
	const chunks = Math.max(1, Math.ceil(cipherLength / (CHUNK + TAG)));
	const plainSize = cipherLength - chunks * TAG;

	const cache = new Map<number, Uint8Array>();
	async function chunk(i: number): Promise<Uint8Array> {
		const hit = cache.get(i);
		if (hit) return hit;
		const start = payloadStart + i * (CHUNK + TAG);
		const len = Math.min(CHUNK + TAG, src.size - start);
		const sealed = await src.read(start, len);
		const n = new Uint8Array(12);
		let c = i;
		for (let b = 10; b >= 0 && c > 0; b--) {
			n[b] = c & 0xff;
			c = Math.floor(c / 256);
		}
		if (i === chunks - 1) n[11] = 1;
		const plain = chacha20poly1305(payloadKey, n).decrypt(sealed);
		if (cache.size > 64) cache.clear();
		cache.set(i, plain);
		return plain;
	}

	return {
		size: plainSize,
		async read(offset, length) {
			const out = new Uint8Array(Math.max(0, Math.min(length, plainSize - offset)));
			let done = 0;
			while (done < out.length) {
				const pos = offset + done;
				const i = Math.floor(pos / CHUNK);
				const plain = await chunk(i);
				const from = pos - i * CHUNK;
				const take = Math.min(plain.length - from, out.length - done);
				out.set(plain.subarray(from, from + take), done);
				done += take;
			}
			return out;
		}
	};
}

export interface ZipEntry {
	name: string;
	method: number;
	compressedSize: number;
	size: number;
	localHeaderOffset: number;
}

function u16(b: Uint8Array, o: number) {
	return b[o] | (b[o + 1] << 8);
}
function u32(b: Uint8Array, o: number) {
	return (b[o] | (b[o + 1] << 8) | (b[o + 2] << 16)) + b[o + 3] * 0x1000000;
}
function u64(b: Uint8Array, o: number) {
	return u32(b, o) + u32(b, o + 4) * 0x1_0000_0000;
}

/** ZIP по оглавлению с конца: записи без чтения файла подряд (zip64 — тоже). */
export class ZipReader {
	private constructor(
		private src: RandomSource,
		readonly entries: Map<string, ZipEntry>
	) {}

	static async open(src: RandomSource): Promise<ZipReader> {
		const tailLen = Math.min(src.size, 65_557 + 20);
		const tail = await src.read(src.size - tailLen, tailLen);
		let eocd = -1;
		for (let i = tail.length - 22; i >= 0; i--) {
			if (u32(tail, i) === 0x06054b50) {
				eocd = i;
				break;
			}
		}
		if (eocd < 0) throw new NotWyndError('no zip directory');
		let count = u16(tail, eocd + 10);
		let cdSize = u32(tail, eocd + 12);
		let cdOffset = u32(tail, eocd + 16);
		// zip64: перед EOCD — локатор, он указывает на запись zip64 EOCD.
		const loc = eocd - 20;
		if (loc >= 0 && u32(tail, loc) === 0x07064b50) {
			const z64At = u64(tail, loc + 8);
			const z64 = await src.read(z64At, 56);
			if (u32(z64, 0) !== 0x06064b50) throw new NotWyndError('zip64');
			count = u64(z64, 32);
			cdSize = u64(z64, 40);
			cdOffset = u64(z64, 48);
		}
		const cd = await src.read(cdOffset, cdSize);
		const entries = new Map<string, ZipEntry>();
		let p = 0;
		for (let n = 0; n < count; n++) {
			if (u32(cd, p) !== 0x02014b50) throw new NotWyndError('zip entry');
			const method = u16(cd, p + 10);
			let compressedSize = u32(cd, p + 20);
			let size = u32(cd, p + 24);
			const nameLen = u16(cd, p + 28);
			const extraLen = u16(cd, p + 30);
			const commentLen = u16(cd, p + 32);
			let localHeaderOffset = u32(cd, p + 42);
			const name = utf8.decode(cd.subarray(p + 46, p + 46 + nameLen));
			// Поле zip64 в extra: только те значения, что в основной записи — 0xFFFFFFFF.
			let e = p + 46 + nameLen;
			const extraEnd = e + extraLen;
			while (e + 4 <= extraEnd) {
				const id = u16(cd, e);
				const len = u16(cd, e + 2);
				if (id === 0x0001) {
					let q = e + 4;
					if (size === 0xffffffff) {
						size = u64(cd, q);
						q += 8;
					}
					if (compressedSize === 0xffffffff) {
						compressedSize = u64(cd, q);
						q += 8;
					}
					if (localHeaderOffset === 0xffffffff) localHeaderOffset = u64(cd, q);
				}
				e += 4 + len;
			}
			entries.set(name, { name, method, compressedSize, size, localHeaderOffset });
			p = extraEnd + commentLen;
		}
		return new ZipReader(src, entries);
	}

	private async dataOffset(e: ZipEntry): Promise<number> {
		const lh = await this.src.read(e.localHeaderOffset, 30);
		if (u32(lh, 0) !== 0x04034b50) throw new NotWyndError('zip local header');
		return e.localHeaderOffset + 30 + u16(lh, 26) + u16(lh, 28);
	}

	/** Запись целиком — для manifest.json и data/*.json. */
	async read(name: string): Promise<Uint8Array> {
		const e = this.entries.get(name);
		if (!e) throw new NotWyndError(`no ${name}`);
		const raw = await this.src.read(await this.dataOffset(e), e.compressedSize);
		if (e.method === 0) return raw;
		if (e.method !== 8) throw new NotWyndError(`method ${e.method}`);
		const input = new ReadableStream<Uint8Array>({
			start(c) {
				c.enqueue(raw);
				c.close();
			}
		});
		const reader = input.pipeThrough(new DecompressionStream('deflate-raw')).getReader();
		const parts: Uint8Array[] = [];
		for (;;) {
			const { done, value } = await reader.read();
			if (done) break;
			parts.push(value);
		}
		return concat(...parts);
	}

	async json<T>(name: string): Promise<T> {
		return JSON.parse(utf8.decode(await this.read(name))) as T;
	}

	/** Медиа (без сжатия) — кусками, чтобы не держать гигабайт в памяти. */
	async *chunks(name: string, size = 1 << 20): AsyncGenerator<Uint8Array> {
		const e = this.entries.get(name);
		if (!e) throw new NotWyndError(`no ${name}`);
		if (e.method !== 0) {
			yield await this.read(name);
			return;
		}
		const start = await this.dataOffset(e);
		for (let done = 0; done < e.size; done += size) {
			yield await this.src.read(start + done, Math.min(size, e.size - done));
		}
	}
}

/** manifest.json полного архива (internal/archive.FullManifest). */
export interface WyndManifest {
	format: string;
	format_version: number;
	wynd_version: string;
	schema_version: number;
	server: string;
	circle_id: string;
	circle_name: string;
	snapshot_at: string;
	posts: number;
	tables: string[];
}

export interface WyndTable {
	columns: string[];
	rows: unknown[][];
}

/** Открыть архив: проверить, что это полный архив круга, и прочитать manifest. */
export async function openArchive(src: RandomSource): Promise<{ zip: ZipReader; manifest: WyndManifest }> {
	const zip = await ZipReader.open(src);
	const manifest = await zip.json<WyndManifest>('manifest.json');
	if (manifest.format !== 'wynd-circle') throw new NotWyndError('format');
	return { zip, manifest };
}
