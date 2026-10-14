import { scryptAsync } from '@noble/hashes/scrypt.js';
import { x25519 } from '@noble/curves/ed25519.js';

/**
 * Ключ шифрования полного архива (план 51). Пароль не уходит с устройства:
 * из него выводится закрытый ключ X25519 (scrypt), серверу идут только
 * открытый ключ и соль — он шифрует на них файл и открыть его не может.
 * scrypt — чистым JS: WebCrypto вне защищённого контекста (Wynd по адресу в
 * локальной сети) нет. Параметры — те же, что в internal/archive/encrypt.go.
 */
export const ARCHIVE_LOG_N = 15;
const SCRYPT_R = 8;
const SCRYPT_P = 1;
const SALT_LEN = 16;

export interface ArchiveKey {
	public_key: string;
	salt: string;
	log_n: number;
}

function toBase64(bytes: Uint8Array): string {
	let s = '';
	for (const b of bytes) s += String.fromCharCode(b);
	return btoa(s);
}

/** Закрытый ключ из пароля — как DeriveKey на сервере. */
export async function derivePrivateKey(
	password: string,
	salt: Uint8Array,
	logN = ARCHIVE_LOG_N,
	onProgress?: (fraction: number) => void
): Promise<Uint8Array> {
	return scryptAsync(new TextEncoder().encode(password), salt, {
		N: 2 ** logN,
		r: SCRYPT_R,
		p: SCRYPT_P,
		dkLen: 32,
		onProgress
	});
}

/** Открытый ключ и соль для сервера; соль — новая на каждый архив. */
export async function deriveArchiveKey(
	password: string,
	onProgress?: (fraction: number) => void,
	salt: Uint8Array = crypto.getRandomValues(new Uint8Array(SALT_LEN)),
	logN = ARCHIVE_LOG_N
): Promise<ArchiveKey> {
	const priv = await derivePrivateKey(password, salt, logN, onProgress);
	const pub = x25519.getPublicKey(priv);
	priv.fill(0);
	return { public_key: toBase64(pub), salt: toBase64(salt), log_n: logN };
}
