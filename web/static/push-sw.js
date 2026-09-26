/* Wynd: показ push-уведомлений. Подключается в сгенерированный service worker
 * через workbox.importScripts (web/vite.config.ts).
 *
 * Сервер шлёт сигнал без текста журнала: тип, круг и счётчик. Текст записи
 * через push-сервисы Google, Mozilla и Apple не ходит — уведомление говорит
 * только, что случилось, а прочитать можно в приложении. Свой текст несут лишь
 * служебные сигналы (подписка, проверка из панели).
 */

const WYND_SIGNAL_TEXT = {
	post: 'Новая запись в круге',
	comment: 'Новый комментарий',
	reaction: 'Новая реакция',
	mention: 'Вас упомянули',
	event: 'В круге изменения'
};

self.addEventListener('push', (event) => {
	let signal = {};
	try {
		signal = event.data ? event.data.json() : {};
	} catch {
		signal = {};
	}
	const title = signal.title || 'Wynd';
	const body = signal.body || WYND_SIGNAL_TEXT[signal.type] || 'Новое в Wynd';
	const url = signal.circle_id ? `/circles/${encodeURIComponent(signal.circle_id)}` : '/';
	// Один тег на круг и тип: пять записей подряд — одно уведомление, не пять.
	const tag = signal.circle_id ? `${signal.circle_id}:${signal.type}` : `wynd:${signal.type || 'signal'}`;
	event.waitUntil(
		self.registration.showNotification(title, {
			body,
			tag,
			renotify: true,
			icon: '/icon-192.png',
			badge: '/icon-192.png',
			data: { url }
		})
	);
});

self.addEventListener('notificationclick', (event) => {
	event.notification.close();
	const url = (event.notification.data && event.notification.data.url) || '/';
	event.waitUntil(
		(async () => {
			const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
			for (const client of windows) {
				if (new URL(client.url).origin !== self.location.origin) continue;
				await client.focus();
				if ('navigate' in client) await client.navigate(url);
				return;
			}
			await self.clients.openWindow(url);
		})()
	);
});
