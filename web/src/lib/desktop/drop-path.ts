// Круг из адреса страницы: файл, перенесённый в открытый круг, становится
// вложением новой записи именно этого круга.
export function circleFromPath(pathname: string): string | null {
	const match = pathname.match(/^\/circles\/([^/]+)/);
	if (!match) return null;
	try {
		return decodeURIComponent(match[1]);
	} catch {
		return match[1];
	}
}
