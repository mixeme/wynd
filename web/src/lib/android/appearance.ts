/** Дополнительная возможность оболочки; браузер и прежний APK продолжают работать. */
export function postAndroidAppearance(status: string, navigation: string): void {
	if (
		typeof window === 'undefined' ||
		![status, navigation].every((c) => c.length === 7 && /^#[0-9a-f]{6}$/i.test(c))
	)
		return;
	const port = (window as unknown as { __wyndAppearance?: { postMessage(message: string): void } })
		.__wyndAppearance;
	if (typeof port?.postMessage !== 'function') return;
	try {
		port.postMessage(JSON.stringify({ status, navigation }));
	} catch {
		/* Страница могла уйти; цвет не влияет на доступность приложения. */
	}
}
