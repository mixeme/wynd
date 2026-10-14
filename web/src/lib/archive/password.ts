import { PASSWORD_WORDS } from './words';

/** Случайное целое в [0, n) без перекоса: лишний хвост отбрасывается. */
function randomBelow(n: number): number {
	const limit = Math.floor(0x1_0000_0000 / n) * n;
	const buf = new Uint32Array(1);
	for (;;) {
		crypto.getRandomValues(buf);
		if (buf[0] < limit) return buf[0] % n;
	}
}

/**
 * Пароль, который приложение предлагает для полного архива (6.29): три
 * разных слова и число, через дефис — «дача-самовар-облако-17». Около 36 бит
 * случайности; ключ из него выводит scrypt, так что подбор дорог.
 */
export function suggestArchivePassword(): string {
	const picked: string[] = [];
	while (picked.length < 3) {
		const w = PASSWORD_WORDS[randomBelow(PASSWORD_WORDS.length)];
		if (!picked.includes(w)) picked.push(w);
	}
	return [...picked, String(10 + randomBelow(90))].join('-');
}
