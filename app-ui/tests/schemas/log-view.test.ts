// The Logs screen's derivation tests (docs/SPEC-UI/001-SPEC-UI.md §6.11, §6.14, §7.6).
//
// Each is a table of input variations (docs/RULLES/TDD.md §2.5): a typical value, a boundary, an empty or
// zero value, a value that would render wrong, and an extreme. The two functions that decide what the
// operator reads are the ones that must not silently return a plausible-looking number.

import { describe, expect, it } from 'vitest';
import {
	bodyTruncated,
	consoleSummary,
	logQuery,
	purgeConfirmed,
	TRUNCATION_MARKER
} from '$lib/schemas/log-view';
import type { LogSearch } from '$lib/schemas/log-search';

function search(overrides: Partial<LogSearch> = {}): LogSearch {
	return {
		period: '24h',
		status: '',
		endpointId: '',
		model: '',
		query: '',
		page: 1,
		notices: [],
		...overrides
	};
}

describe('logQuery', () => {
	const now = new Date('2026-09-17T00:00:00Z');

	it('sends the period as a from and to pair, with no blank filters', () => {
		const q = logQuery(search(), now);

		expect(q.from).toBe('2026-09-16T00:00:00Z');
		expect(q.to).toBe('2026-09-17T00:00:00Z');
		expect(q.status).toBeUndefined();
		expect(q.endpoint_id).toBeUndefined();
		expect(q.model).toBeUndefined();
		expect(q.q).toBeUndefined();
	});

	it('anchors "today" to the UTC calendar day, not to a 24 hour span', () => {
		const q = logQuery(search({ period: 'today', status: 'error', endpointId: 'ep_1' }), now);

		expect(q.from).toBe('2026-09-17T00:00:00Z');
		expect(q.to).toBe('2026-09-17T00:00:00Z');
		expect(q.status).toBe('error');
		expect(q.endpoint_id).toBe('ep_1');
	});

	it('keeps page and the page size the panel reads', () => {
		const q = logQuery(search({ page: 3, model: 'gpt-4o', query: 'timeout' }), now);

		expect(q.page).toBe(3);
		expect(q.per_page).toBe(25);
		expect(q.model).toBe('gpt-4o');
		expect(q.q).toBe('timeout');
	});
});

describe('bodyTruncated', () => {
	const cases = [
		{
			name: 'recognizes the marker at the end',
			input: `{"ok":true}${TRUNCATION_MARKER}`,
			expected: true
		},
		{ name: 'a clean body', input: '{"ok":true}', expected: false },
		{ name: 'no body at all', input: undefined, expected: false },
		{
			name: 'the marker in the middle is not a truncation',
			input: `a${TRUNCATION_MARKER}b`,
			expected: false
		},
		{ name: 'an empty body', input: '', expected: false }
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			expect(bodyTruncated(testCase.input as string | undefined)).toBe(testCase.expected);
		});
	}
});

describe('consoleSummary', () => {
	const cases = [
		{ name: 'a single line', lines: 1, max: 1000, text: '1 line held, up to 1,000.' },
		{ name: 'zero lines', lines: 0, max: 1000, text: '0 lines held, up to 1,000.' },
		{ name: 'a typical count', lines: 250, max: 1000, text: '250 lines held, up to 1,000.' },
		{
			name: 'a buffer at its ceiling',
			lines: 1000,
			max: 1000,
			text: '1,000 lines held, up to 1,000.'
		},
		{
			name: 'a buffer beyond the ceiling is not claimed',
			lines: 4000,
			max: 1000,
			text: '4,000 lines held, up to 1,000.'
		}
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			expect(consoleSummary(testCase.lines, testCase.max)).toBe(testCase.text);
		});
	}
});

describe('purgeConfirmed', () => {
	const cases = [
		{ name: 'the exact word', input: 'purge', expected: true },
		{ name: 'case is not the point, the word is', input: 'PURGE', expected: true },
		{ name: 'surrounding space is trimmed', input: ' purge ', expected: true },
		{ name: 'a prefix is not the word', input: 'pur', expected: false },
		{ name: 'an empty field', input: '', expected: false },
		{ name: 'another word entirely', input: 'yes', expected: false }
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			expect(purgeConfirmed(testCase.input)).toBe(testCase.expected);
		});
	}
});
