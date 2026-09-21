// Tests for response parsing (src/lib/api/parse.ts).
//
// Parsing decides two things per response: whether the body is usable, and which unknown top-level
// keys the API added. Both are tables, so a new drift case is a row rather than another copy of the
// same setup. The client's own mapping is covered in tests/api/client.test.ts.

import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import { parseResponse, unknownTopLevelKeys } from '$lib/api/parse';

const schemaThing = z.object({ id: z.string(), count: z.number().int().min(0) });

describe('unknownTopLevelKeys', () => {
	const cases = [
		{ name: 'no drift', input: { id: 'a', count: 1 }, expected: [] },
		{ name: 'one added field', input: { id: 'a', count: 1, extra: true }, expected: ['extra'] },
		{
			name: 'two added fields',
			input: { id: 'a', count: 1, extra: 1, note: 'x' },
			expected: ['extra', 'note']
		},
		{ name: 'an array payload', input: [1, 2], expected: [] },
		{ name: 'a null payload', input: null, expected: [] },
		{ name: 'a primitive payload', input: 'text', expected: [] }
	];

	for (const testCase of cases) {
		it(`reports ${testCase.name}`, () => {
			expect(unknownTopLevelKeys(schemaThing, testCase.input)).toEqual(testCase.expected);
		});
	}
});

describe('parseResponse', () => {
	const cases = [
		{ name: 'a valid body', input: { id: 'a', count: 0 }, ok: true },
		{ name: 'an additive field', input: { id: 'a', count: 3, note: 'new' }, ok: true },
		{ name: 'a renamed field', input: { id: 'a', total: 3 }, ok: false },
		{ name: 'a wrong type', input: { id: 'a', count: 'three' }, ok: false },
		{ name: 'a negative count', input: { id: 'a', count: -3 }, ok: false },
		{ name: 'a missing body', input: null, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(parseResponse(schemaThing, testCase.input).ok).toBe(testCase.ok);
		});
	}

	const pathCases = [
		{ name: 'a renamed field', input: { id: 'a', total: 3 }, path: 'count' },
		{ name: 'a wrong type', input: { id: 'a', count: 'three' }, path: 'count' },
		{ name: 'a missing field', input: { count: 1 }, path: 'id' },
		{ name: 'a null body', input: null, path: '(root)' },
		{ name: 'an array body', input: [1, 2], path: '(root)' }
	];

	for (const testCase of pathCases) {
		it(`names ${testCase.path} as the offending path for ${testCase.name}`, () => {
			const result = parseResponse(schemaThing, testCase.input);

			expect(result.ok).toBe(false);
			if (!result.ok) expect(result.path).toBe(testCase.path);
		});
	}

	const driftCases = [
		{ name: 'no added field', input: { id: 'a', count: 1 }, drift: [] },
		{ name: 'one added field', input: { id: 'a', count: 1, extra: 1 }, drift: ['extra'] },
		{
			name: 'two added fields',
			input: { id: 'a', count: 1, extra: 1, note: 'x' },
			drift: ['extra', 'note']
		}
	];

	for (const testCase of driftCases) {
		it(`reports drift for ${testCase.name} alongside parsed data`, () => {
			const result = parseResponse(schemaThing, testCase.input);

			expect(result.ok).toBe(true);
			if (result.ok) expect(result.drift).toEqual(testCase.drift);
		});
	}
});
