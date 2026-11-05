import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkProject, formatReport } from './ui-guard.mjs';
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
