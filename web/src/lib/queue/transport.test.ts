import { describe, expect, it } from 'vitest';
import { ApiError } from '$lib/api/client';
import { isTransportError } from './transport';

describe('isTransportError', () => {
	it('treats non-ApiError as transport', () => {
		expect(isTransportError(new TypeError('Failed to fetch'))).toBe(true);
		expect(isTransportError(new Error('network'))).toBe(true);
	});

	it('treats ApiError as non-transport', () => {
		expect(isTransportError(new ApiError(400, 'invalid'))).toBe(false);
		expect(isTransportError(new ApiError(403, 'forbidden'))).toBe(false);
		expect(isTransportError(new ApiError(500, 'internal'))).toBe(false);
		expect(isTransportError(new ApiError(502, 'bad_gateway'))).toBe(false);
	});

	it('does not queue an aborted request', () => {
		const err = new Error('aborted');
		err.name = 'AbortError';
		expect(isTransportError(err)).toBe(false);
	});
});
