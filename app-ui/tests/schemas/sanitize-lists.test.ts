// Tests for the transforms that return a list or a flag (src/lib/schemas/sanitize.ts).
//
// Split from tests/schemas/sanitize.test.ts so each file stays inside the project line limit, and
// because these two do not transform a string into another string: one splits a value into hosts and
// the other answers a question about one. Both use the same table helper.

import { describe, expect } from 'vitest';
import { hasAngleBrackets, normalizeNoProxyList } from '$lib/schemas/sanitize';
import { forEachCase } from '../support/tables';

type ListCase = { name: string; input: string; expected: string[] };
type FlagCase = { name: string; input: string; expected: boolean };

describe('normalizeNoProxyList', () => {
	forEachCase<ListCase>(
		[
			{ name: 'splits a typical list', input: 'a.com,b.com', expected: ['a.com', 'b.com'] },
			{
				name: 'trims, lowercases, and deduplicates',
				input: 'a.com, B.com ,, a.com',
				expected: ['a.com', 'b.com']
			},
			{ name: 'handles an empty value', input: '', expected: [] },
			{ name: 'reduces separators only to an empty list', input: ',,, ', expected: [] },
			{ name: 'drops a host that is only junk', input: 'a\u0000.com,', expected: ['a.com'] },
			{
				name: 'keeps an entry past the schema limit for the schema to reject',
				input: `${'h'.repeat(253)}.com`,
				expected: [`${'h'.repeat(253)}.com`]
			}
		],
		(testCase) => expect(normalizeNoProxyList(testCase.input)).toEqual(testCase.expected)
	);
});

describe('hasAngleBrackets', () => {
	forEachCase<FlagCase>(
		[
			{ name: 'reports false for plain text', input: 'plain', expected: false },
			{ name: 'reports true for a tag', input: '<b>', expected: true },
			{ name: 'reports true for a closing bracket alone', input: '>', expected: true },
			{ name: 'reports true for a mixed value', input: 'a<b', expected: true },
			{ name: 'reports false for an empty value', input: '', expected: false }
		],
		(testCase) => expect(hasAngleBrackets(testCase.input)).toBe(testCase.expected)
	);
});
