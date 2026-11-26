import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkProject, formatReport } from './ui-guard.mjs';
import { checkChromeOwners } from './chrome-guard.mjs';
import { checkViewportMeta } from './viewport-guard.mjs';
import { checkWorkshopBaselineFiles } from '../../docs/visual/workshop-baseline-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const result = checkProject(webRoot);
const report = formatReport(result);
if (!result.ok) {
	console.error(report);
	process.exit(1);
}
console.log(report);
const workshopBaseline = checkWorkshopBaselineFiles();
console.log(
	`workshop-baseline: ${workshopBaseline.pinnedFrames} approved base; ${workshopBaseline.pendingVisualReview.length} layouts awaiting visual acceptance (not approved)`
);
// Шапки (C140): структурный класс — у своего компонента, не копия в чужом.
const chromeHits = checkChromeOwners(webRoot);
if (chromeHits.length) {
	console.error(
		'chrome-guard: класс шапки вне своего компонента — шапку собирает chrome/ или layout, не копия.\n' +
			chromeHits.join('\n')
	);
	process.exit(1);
}
console.log('chrome-guard: OK (шапки у своих компонентов)');
// Клавиатура (C144): layout viewport не сжимает никто, кроме страницы.
const viewportHits = checkViewportMeta(webRoot);
if (viewportHits.length) {
	console.error(
		'viewport-guard: interactive-widget=resizes-content возвращает белую вспышку на клавиатуре — ' +
			'высоту окна считает session/viewport.ts по visualViewport.\n' +
			viewportHits.join('\n')
	);
	process.exit(1);
}
console.log('viewport-guard: OK (клавиатура не сжимает layout viewport)');
// Лесенка вертикальных зазоров (C120) — часть проверки интерфейса.
await import('./check-spacing.mjs');
