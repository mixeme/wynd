import prettier from 'eslint-config-prettier';
import js from '@eslint/js';
import { includeIgnoreFile } from '@eslint/compat';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import { fileURLToPath } from 'node:url';
import ts from 'typescript-eslint';
import svelteConfig from './svelte.config.js';

const gitignorePath = fileURLToPath(new URL('./.gitignore', import.meta.url));

export default ts.config(
	includeIgnoreFile(gitignorePath),
	js.configs.recommended,
	...ts.configs.recommended,
	...svelte.configs['flat/recommended'],
	prettier,
	...svelte.configs['flat/prettier'],
	{
		languageOptions: { globals: { ...globals.browser, ...globals.node } },
		rules: {
			// Имена типов (MediaTrackConstraints, параметры generics) проверяет TypeScript.
			'no-undef': 'off',
			// Неразрывные пробелы в строках интерфейса («50 %») — нарочно.
			'no-irregular-whitespace': ['error', { skipStrings: true, skipTemplates: true }],
			// _имя — нарочно не используется: отброшенное при разборе, аргумент заглушки.
			'@typescript-eslint/no-unused-vars': [
				'error',
				{ argsIgnorePattern: '^_', varsIgnorePattern: '^_', ignoreRestSiblings: true }
			]
		}
	},
	{
		files: ['**/*.svelte', '**/*.svelte.ts'],
		languageOptions: { parserOptions: { parser: ts.parser, svelteConfig } },
		rules: {
			// Плагин 2.x компилирует каждый компонент как custom element и ругается на
			// $props() с остатком — Wynd их не собирает. Предупреждения компилятора
			// и так показывает svelte-check; здесь — только ошибки компиляции.
			'svelte/valid-compile': ['error', { ignoreWarnings: true }]
		}
	},
	{
		// Объявления для всего проекта: «не используется» здесь не значит ничего.
		files: ['**/*.d.ts'],
		rules: { '@typescript-eslint/no-unused-vars': 'off' }
	},
	{
		// Node-скрипты без типов (ui-guard.mjs): @ts-nocheck — нарочно.
		files: ['scripts/**'],
		rules: { '@typescript-eslint/ban-ts-comment': ['error', { 'ts-nocheck': false }] }
	}
);
