import { describe, expect, it } from 'vitest';
import { takeSSEDataEvents } from './sync';

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
});
