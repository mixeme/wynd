/// <reference types="vite-plugin-pwa/info" />
/// <reference types="vite-plugin-pwa/client" />

declare const __WYND_VERSION__: string;

declare global {
	namespace App {}
	interface Window {
		/** Временный журнал запуска из app.html (план 46). */
		__wyndBoot?: (what: string) => void;
	}
}

export {};
