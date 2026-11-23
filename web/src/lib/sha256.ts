/**
 * SHA-256 файла для завершения загрузки (`POST /uploads/{id}/complete`).
 *
 * `crypto.subtle` есть только в secure context. Локальный запуск, открытый с
 * телефона по `http://192.168.…`, его не имеет, и без запасного пути ни одна
 * фотография не уходила. Запасной путь медленнее и нужен только там.
 *
 * Крупные файлы считаются потоком: `createSha256` обновляется теми же
 * порциями, что уходят на сервер (C119). Один `crypto.subtle.digest` на
 * 94 МБ в WebView дал неверный SHA при верных отправленных байтах.
 */
const SMALL_DIGEST_LIMIT = 8 * 1024 * 1024;

export async function sha256Hex(data: ArrayBuffer): Promise<string> {
	const subtle = typeof crypto !== 'undefined' ? crypto.subtle : undefined;
	if (subtle && data.byteLength <= SMALL_DIGEST_LIMIT) {
		const hash = new Uint8Array(await subtle.digest('SHA-256', data));
		return hex(hash);
	}
	const stream = createSha256();
	stream.update(new Uint8Array(data));
	return stream.hex();
}

/** Потоковый SHA-256: `update` можно звать кусками, `hex` — один раз в конце. */
export interface Sha256Stream {
	update(data: ArrayBuffer | Uint8Array): void;
	digest(): Uint8Array;
	hex(): string;
}

const K = new Uint32Array([
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2
]);

export function createSha256(): Sha256Stream {
	const h = new Uint32Array([
		0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19
	]);
	const w = new Uint32Array(64);
	const pending = new Uint8Array(64);
	let pendingLen = 0;
	let total = 0;

	function block(v: DataView, off: number) {
		for (let i = 0; i < 16; i++) w[i] = v.getUint32(off + i * 4);
		for (let i = 16; i < 64; i++) {
			const a = w[i - 15];
			const b = w[i - 2];
			const s0 = ((a >>> 7) | (a << 25)) ^ ((a >>> 18) | (a << 14)) ^ (a >>> 3);
			const s1 = ((b >>> 17) | (b << 15)) ^ ((b >>> 19) | (b << 13)) ^ (b >>> 10);
			w[i] = (w[i - 16] + s0 + w[i - 7] + s1) | 0;
		}
		let [a, b, c, d, e, f, g, hh] = h;
		for (let i = 0; i < 64; i++) {
			const S1 = ((e >>> 6) | (e << 26)) ^ ((e >>> 11) | (e << 21)) ^ ((e >>> 25) | (e << 7));
			const ch = (e & f) ^ (~e & g);
			const t1 = (hh + S1 + ch + K[i] + w[i]) | 0;
			const S0 = ((a >>> 2) | (a << 30)) ^ ((a >>> 13) | (a << 19)) ^ ((a >>> 22) | (a << 10));
			const maj = (a & b) ^ (a & c) ^ (b & c);
			const t2 = (S0 + maj) | 0;
			hh = g;
			g = f;
			f = e;
			e = (d + t1) | 0;
			d = c;
			c = b;
			b = a;
			a = (t1 + t2) | 0;
		}
		h[0] += a;
		h[1] += b;
		h[2] += c;
		h[3] += d;
		h[4] += e;
		h[5] += f;
		h[6] += g;
		h[7] += hh;
	}

	function update(data: ArrayBuffer | Uint8Array) {
		const bytes = data instanceof Uint8Array ? data : new Uint8Array(data);
		total += bytes.length;
		let at = 0;
		if (pendingLen > 0) {
			const take = Math.min(64 - pendingLen, bytes.length);
			pending.set(bytes.subarray(0, take), pendingLen);
			pendingLen += take;
			at = take;
			if (pendingLen === 64) {
				block(new DataView(pending.buffer), 0);
				pendingLen = 0;
			}
		}
		const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
		for (; at + 64 <= bytes.length; at += 64) block(dv, at);
		if (at < bytes.length) {
			pending.set(bytes.subarray(at));
			pendingLen = bytes.length - at;
		}
	}

	function digest(): Uint8Array {
		// Хвост: остаток данных, 0x80, нули и длина в битах — один или два блока.
		const tailLen = pendingLen < 56 ? 64 : 128;
		const tail = new Uint8Array(tailLen);
		tail.set(pending.subarray(0, pendingLen));
		tail[pendingLen] = 0x80;
		const tv = new DataView(tail.buffer);
		const bits = total * 8;
		tv.setUint32(tailLen - 8, Math.floor(bits / 0x100000000));
		tv.setUint32(tailLen - 4, bits >>> 0);
		for (let off = 0; off < tailLen; off += 64) block(tv, off);
		const out = new Uint8Array(32);
		const ov = new DataView(out.buffer);
		for (let i = 0; i < 8; i++) ov.setUint32(i * 4, h[i]);
		return out;
	}

	return {
		update,
		digest,
		hex: () => hex(digest())
	};
}

/** Чистый SHA-256 (FIPS 180-4) — запасной путь без `crypto.subtle`. */
export function sha256(data: Uint8Array): Uint8Array {
	const stream = createSha256();
	stream.update(data);
	return stream.digest();
}

function hex(hash: Uint8Array): string {
	return [...hash].map((b) => b.toString(16).padStart(2, '0')).join('');
}
