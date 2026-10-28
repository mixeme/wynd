/// <reference types="vite-plugin-pwa/info" />
/// <reference types="vite-plugin-pwa/client" />

declare const __WYND_VERSION__: string;

declare global {
	namespace App {
		interface PageState {
			/** Заголовок «Доступ закрыт» одного сервера — имя круга, с которого пришли (10.17). */
			closedTitle?: string;
		}
	}
	interface Window {
		/** Временный журнал запуска из app.html (план 46). */
		__wyndBoot?: (what: string) => void;
	}
}

export {};
