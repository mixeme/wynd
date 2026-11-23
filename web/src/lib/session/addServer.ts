// Второй сервер добавляют только приложения — окно для компьютера и Android
// (C69 решён 23.11: в PWA многосерверности не будет; C123). В браузере
// кнопки нет, хотя обновлённый сервер уже разрешает запросы участника с
// другого адреса: продукт в PWA второй сервер не предлагает. «Войти на
// сервер» в браузере на старом сервере без CORS отвечал бы «Сервер не
// отвечает».
import { androidPort } from '$lib/android/bridge';
import { desktopBridge } from '$lib/desktop/bridge';

export const ADD_SERVER_IN_APP =
	typeof window !== 'undefined' && (Boolean(desktopBridge()) || Boolean(androidPort()));
