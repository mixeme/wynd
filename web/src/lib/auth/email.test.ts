import { describe, expect, it } from 'vitest';
import { isValidParticipantEmail } from './email';

describe('isValidParticipantEmail', () => {
	it('accepts a mailbox with a dotted domain', () => {
		expect(isValidParticipantEmail('ana@example.com')).toBe(true);
		expect(isValidParticipantEmail('  Ana@Example.COM  ')).toBe(true);
		expect(isValidParticipantEmail('Ana <ana@example.com>')).toBe(true);
	});

	it('rejects a string without @ and domain dot', () => {
		expect(isValidParticipantEmail('no-at')).toBe(false);
		expect(isValidParticipantEmail('@example.com')).toBe(false);
		expect(isValidParticipantEmail('ana@')).toBe(false);
		expect(isValidParticipantEmail('ana@localhost')).toBe(false);
		expect(isValidParticipantEmail('')).toBe(false);
	});
});
