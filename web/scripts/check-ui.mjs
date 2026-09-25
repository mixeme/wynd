import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkProject, formatReport } from './ui-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const result = checkProject(webRoot);
const report = formatReport(result);
if (!result.ok) {
	console.error(report);
	process.exit(1);
}
console.log(report);
