// Time formatting tests (docs/SPEC-UI/001-SPEC-UI.md §7.2).
//
// The countdown is the only figure on the quota screen that changes without a request, so its boundaries
// are the ones an operator will actually watch: the minute, the hour, and the day rollover. It also has
// to stay honest about a reset instant that has already passed, which means the worker has not refreshed
// the row yet rather than that the window is overdue.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { countdownText, formatTimestamp } from '$lib/utils/time';

const NOW = Date.parse('2026-09-18T12:00:00Z');

function at(offsetMs: number): string {
	return new Date(NOW + offsetMs).toISOString();
}

describe('countdownText', () => {
	forEachCase(
		[
			{
				name: 'says any moment now for an instant that has passed',
				offset: -60_000,
				expected: 'any moment now'
			},
			{ name: 'says any moment now for the exact instant', offset: 0, expected: 'any moment now' },
			{
				name: 'counts a second as under a minute rather than as 0m',
				offset: 1_000,
				expected: 'in under a minute'
			},
			{
				name: 'counts the last second before a minute',
				offset: 59_000,
				expected: 'in under a minute'
			},
			{ name: 'counts a whole minute', offset: 60_000, expected: 'in 1m' },
			{ name: 'drops the seconds rather than rounding up', offset: 119_000, expected: 'in 1m' },
			{ name: 'counts the last minute before an hour', offset: 3_599_000, expected: 'in 59m' },
			{ name: 'counts a whole hour', offset: 3_600_000, expected: 'in 1h' },
			{ name: 'counts an hour and a half', offset: 5_400_000, expected: 'in 1h 30m' },
			{ name: 'counts the last hour before a day', offset: 86_340_000, expected: 'in 23h 59m' },
			{ name: 'counts a whole day', offset: 86_400_000, expected: 'in 1d' },
			{ name: 'counts a day and a half', offset: 129_600_000, expected: 'in 1d 12h' },
			{ name: 'counts a long window in days', offset: 30 * 86_400_000, expected: 'in 30d' }
		],
		(testCase) => {
			expect(countdownText(at(testCase.offset), NOW)).toBe(testCase.expected);
		}
	);

	it('never renders a negative duration, whatever the instant', () => {
		for (const offset of [-1, -1_000, -86_400_000, -365 * 86_400_000]) {
			expect(countdownText(at(offset), NOW)).toBe('any moment now');
		}
	});

	it('states an unparseable instant instead of inventing a duration', () => {
		expect(countdownText('not a timestamp', NOW)).toBe('Invalid timestamp');
	});
});

describe('formatTimestamp', () => {
	it('states an unparseable value rather than rendering a native invalid date', () => {
		expect(formatTimestamp('2026-13-45')).toBe('Invalid timestamp');
	});

	it('names the zone it rendered in, so the value is not read as UTC', () => {
		// The zone is whatever the runtime is in, so the assertion is on the shape: a zone token after the
		// date, which is what stops a reader from taking a local time for the API's UTC.
		expect(formatTimestamp('2026-09-18T12:00:00Z')).toMatch(/,\s*\S+$/);
	});
});
