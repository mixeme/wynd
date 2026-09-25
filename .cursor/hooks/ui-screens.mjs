import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { stdin, stdout } from 'node:process';
import {
	analyzeScreenFile,
	analyzeScreenSource,
	checkProject,
	classifyNewSvelte,
	formatReport,
	libraryInventory,
	posixRel,
	UI_GAP_PLAN_HINT
} from '../../web/scripts/ui-guard.mjs';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const webRoot = path.join(repoRoot, 'web');

function out(payload) {
	stdout.write(`${JSON.stringify(payload)}\n`);
}

async function readStdin() {
	const chunks = [];
	for await (const chunk of stdin) chunks.push(chunk);
	const raw = Buffer.concat(chunks).toString('utf8').trim();
	if (!raw) return {};
	try {
		return JSON.parse(raw);
	} catch {
		return {};
	}
}

function toolInput(event) {
	const raw = event.tool_input;
	if (raw && typeof raw === 'object') return raw;
	if (typeof raw === 'string') {
		try {
			return JSON.parse(raw);
		} catch {
			return {};
		}
	}
	return {};
}

function toolName(event) {
	return String(event.tool_name ?? event.tool ?? '');
}

function writePath(event) {
	if (event.file_path) return resolvePath(event.file_path);
	const input = toolInput(event);
	return resolvePath(input.path ?? input.file_path ?? input.target_notebook ?? '');
}

function resolvePath(p) {
	if (!p) return '';
	return path.isAbsolute(p) ? path.resolve(String(p)) : path.resolve(repoRoot, String(p));
}

function writeContents(event) {
	const input = toolInput(event);
	if (typeof input.contents === 'string') return input.contents;
	if (typeof input.new_string === 'string') return input.new_string;
	const edits = event.edits;
	if (Array.isArray(edits) && edits.length) {
		return edits.map((e) => e?.new_string ?? '').join('\n');
	}
	return '';
}

function isWriteLike(name) {
	return /Write|StrReplace|TabWrite/i.test(name);
}

function webRelFromAbs(absPath) {
	if (!absPath) return null;
	return posixRel(webRoot, path.resolve(absPath));
}

function screenContext(hits) {
	return [
		'Wynd UI screen guard: this screen must be assembled only from existing $ui components and $lib/layouts.',
		'Do not invent markup primitives, Bits UI, or new .svelte files. Fix the hits, then continue.',
		UI_GAP_PLAN_HINT,
		hits.join('\n')
	].join('\n');
}

function handlePreToolUse(event) {
	if (!isWriteLike(toolName(event))) return {};
	const abs = writePath(event);
	if (!abs) return {};
	const rel = webRelFromAbs(abs);
	if (!rel) return {};
	const exists = fs.existsSync(path.resolve(abs));
	const created = classifyNewSvelte(rel, exists, repoRoot);
	if (created?.deny) {
		return {
			permission: 'deny',
			user_message: created.message,
			agent_message: created.message
		};
	}
	return {};
}

function hitsForEdit(event) {
	const abs = writePath(event);
	if (!abs) return [];
	const rel = webRelFromAbs(abs);
	if (!rel) return [];
	const inventory = libraryInventory(webRoot);
	if (fs.existsSync(path.resolve(abs))) {
		return analyzeScreenFile(path.resolve(abs), webRoot, inventory);
	}
	const contents = writeContents(event);
	if (!contents) return [];
	return analyzeScreenSource(rel, contents, inventory);
}

function handleAfterEdit(event) {
	const hits = hitsForEdit(event);
	if (!hits.length) return {};
	return { additional_context: screenContext(hits) };
}

function handleStop(event) {
	if (event.status && event.status !== 'completed') return {};
	if (Number(event.loop_count) > 0) return {};
	const result = checkProject(webRoot);
	if (result.ok) return {};
	return {
		followup_message: [
			'Wynd UI screen guard failed. Screens must be assembled only from existing $ui components and $lib/layouts.',
			UI_GAP_PLAN_HINT,
			formatReport(result)
		].join('\n\n')
	};
}

const event = await readStdin();
const name = String(event.hook_event_name ?? '');

let payload = {};
try {
	if (name === 'preToolUse') payload = handlePreToolUse(event);
	else if (name === 'postToolUse' || name === 'afterFileEdit' || name === 'afterTabFileEdit') {
		payload = handleAfterEdit(event);
	} else if (name === 'stop') payload = handleStop(event);
} catch {
	payload = {};
}

out(payload);
