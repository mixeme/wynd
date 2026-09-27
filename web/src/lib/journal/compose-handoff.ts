// Фото из строки ввода на 3.1 до экрана записи 4.2. Пикер открывается там,
// где нажали: вызов input.click() после перехода браузер может не пустить —
// жест пользователя к тому времени уже истёк. Файлы живут только в памяти
// вкладки, до первого чтения; ключ — круг, чтобы чужой круг их не подхватил.

let pending: { circleId: string; files: File[] } | null = null;

export function handComposePhotos(circleId: string, files: File[]): void {
	pending = files.length ? { circleId, files } : null;
}

export function takeComposePhotos(circleId: string): File[] {
	const got = pending?.circleId === circleId ? pending.files : [];
	pending = null;
	return got;
}
