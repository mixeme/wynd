import { describe, expect, it } from 'vitest';
import { x25519 } from '@noble/curves/ed25519.js';
import { deriveArchiveKey, derivePrivateKey } from './fullkey';
import { suggestArchivePassword } from './password';
import { PASSWORD_WORDS } from './words';

const hex = (b: Uint8Array) => [...b].map((x) => x.toString(16).padStart(2, '0')).join('');

describe('ключ полного архива', () => {
	// Тот же пример, что TestDeriveKeyMatchesDevice в internal/archive: ключ из
	// пароля устройство и сервер выводят одинаково.
	it('совпадает с сервером', async () => {
		const salt = new TextEncoder().encode('wynd-test-salt16');
		const priv = await derivePrivateKey('дача-самовар-облако-17', salt, 15);
		expect(hex(priv)).toBe('5d7553f7c60edfb2fee4e2af90b1e3925f5d477d2db1d4f605c1d5d4f4adec2d');
		expect(hex(x25519.getPublicKey(priv))).toBe(
			'fc2e4afacf6e841679da98516ae8bf3f5c3abf0eb163f63ba693e7dd75f99735'
		);
	}, 30_000);

	it('соль новая на каждый архив', async () => {
		const a = await deriveArchiveKey('пароль', undefined, undefined, 10);
		const b = await deriveArchiveKey('пароль', undefined, undefined, 10);
		expect(a.salt).not.toBe(b.salt);
		expect(a.public_key).not.toBe(b.public_key);
		expect(atob(a.public_key)).toHaveLength(32);
	});
});

describe('пароль полного архива', () => {
	it('три разных слова из словаря и число', () => {
		for (let i = 0; i < 50; i++) {
			const parts = suggestArchivePassword().split('-');
			expect(parts).toHaveLength(4);
			expect(new Set(parts.slice(0, 3)).size).toBe(3);
			for (const w of parts.slice(0, 3)) expect(PASSWORD_WORDS).toContain(w);
			expect(Number(parts[3])).toBeGreaterThanOrEqual(10);
			expect(Number(parts[3])).toBeLessThan(100);
		}
	});

	it('словарь без повторов и только строчная кириллица', () => {
		expect(new Set(PASSWORD_WORDS).size).toBe(PASSWORD_WORDS.length);
		expect(PASSWORD_WORDS.length).toBeGreaterThanOrEqual(1024);
		for (const w of PASSWORD_WORDS) expect(w).toMatch(/^[а-яё]+$/);
	});
});
