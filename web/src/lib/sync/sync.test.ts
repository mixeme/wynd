import { describe, expect, it } from 'vitest';
import { retryDelayMs, takeSSEDataEvents } from './sync';

describe('sync SSE', () => {
	it('extracts data payloads from blocks delimited by blank lines', () => {
		const chunk =
			'event: message\ndata: {"seq":1}\n\n' +
			': comment\n\ndata: {"seq":2}\n\n' +
			'data: {"seq":3}\n\npartial';
		const { events, rest } = takeSSEDataEvents(chunk);
		expect(events).toEqual(['{"seq":1}', '{"seq":2}', '{"seq":3}']);
		expect(rest).toBe('partial');
	});

	// Прокси и серверы переводят строки по-разному; парсер обязан читать поток
	// по спецификации, а не только LF с пробелом после двоеточия (API-3).
	it('reads CRLF frames and data without a space after the colon', () => {
		const chunk = 'event: sync\r\ndata:{"seq":7}\r\n\r\n';
		const { events, rest } = takeSSEDataEvents(chunk);
		expect(events).toEqual(['{"seq":7}']);
		expect(rest).toBe('');
	});

	it('joins multi-line data fields', () => {
		const { events } = takeSSEDataEvents('data: {"a":1,\ndata: "b":2}\n\n');
		expect(events).toEqual(['{"a":1,\n"b":2}']);
	});

	it('treats a heartbeat comment as no event', () => {
		const { events, rest } = takeSSEDataEvents(':\n\n');
		expect(events).toEqual([]);
		expect(rest).toBe('');
	});
});

describe('sync backoff', () => {
	// Фиксированные 5 с превращали недоступный сервер в непрерывный опрос;
	// джиттер разносит повторы нескольких вкладок (API-3).
	it('doubles up to a minute and stays within the jitter band', () => {
		expect(retryDelayMs(0, () => 0.5)).toBe(5000);
		expect(retryDelayMs(1, () => 0.5)).toBe(10000);
		expect(retryDelayMs(4, () => 0.5)).toBe(60000);
		expect(retryDelayMs(10, () => 0.5)).toBe(60000);
		expect(retryDelayMs(0, () => 0)).toBe(4000);
		expect(retryDelayMs(0, () => 1)).toBe(6000);
	});
});
