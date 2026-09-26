// Polling tests (docs/SPEC-UI/001-SPEC-UI.md §8.6.1).
//
// The due check decides whether a screen asks the gateway for data nobody is reading. It is separated from
// the timer so those three conditions can be exercised without waiting thirty seconds or mounting a
// component: paused, hidden, and not yet due each have to win over the elapsed interval.

import { describe, expect } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	clampPage,
	pollDue,
	pollIntervalLabel,
	QUOTA_PAGE_SIZE,
	QUOTA_POLL_MS
} from '$lib/polling';

describe('pollIntervalLabel', () => {
	forEachCase(
		[
			{ name: 'names a one second interval', ms: 1_000, expected: 'every second' },
			{ name: 'names a five second interval', ms: 5_000, expected: 'every 5 seconds' },
			{ name: 'names the quota interval', ms: QUOTA_POLL_MS, expected: 'every 30 seconds' },
			{ name: 'names a minute in the singular', ms: 60_000, expected: 'every minute' },
			{ name: 'names two minutes', ms: 120_000, expected: 'every 2 minutes' },
			{
				name: 'leaves an interval that is not a whole minute in seconds',
				ms: 90_000,
				expected: 'every 90 seconds'
			}
		],
		(testCase) => {
			expect(pollIntervalLabel(testCase.ms)).toBe(testCase.expected);
		}
	);
});

describe('pollDue', () => {
	const visible = 'visible';
	const loaded = 1_000_000;

	forEachCase(
		[
			{
				name: 'is due once the interval has elapsed',
				state: { paused: false, lastLoadedAt: loaded },
				now: loaded + QUOTA_POLL_MS,
				visibility: visible,
				expected: true
			},
			{
				name: 'is due after the interval has passed',
				state: { paused: false, lastLoadedAt: loaded },
				now: loaded + QUOTA_POLL_MS + 5_000,
				visibility: visible,
				expected: true
			},
			{
				name: 'is not due one millisecond before the interval elapses',
				state: { paused: false, lastLoadedAt: loaded },
				now: loaded + QUOTA_POLL_MS - 1,
				visibility: visible,
				expected: false
			},
			{
				name: 'is never due while paused, however long it has been',
				state: { paused: true, lastLoadedAt: loaded },
				now: loaded + 10 * QUOTA_POLL_MS,
				visibility: visible,
				expected: false
			},
			{
				name: 'is not due while the tab is hidden, so a background tab stops asking',
				state: { paused: false, lastLoadedAt: loaded },
				now: loaded + 10 * QUOTA_POLL_MS,
				visibility: 'hidden',
				expected: false
			},
			{
				name: 'is not due for a document state the panel does not recognise',
				state: { paused: false, lastLoadedAt: loaded },
				now: loaded + 10 * QUOTA_POLL_MS,
				visibility: 'prerender',
				expected: false
			},
			{
				name: 'is due when nothing has been read yet',
				state: { paused: false, lastLoadedAt: 0 },
				now: Date.parse('2026-09-18T12:00:00Z'),
				visibility: visible,
				expected: true
			},
			{
				name: 'is still not due before its first read while the tab is hidden',
				state: { paused: false, lastLoadedAt: 0 },
				now: Date.parse('2026-09-18T12:00:00Z'),
				visibility: 'hidden',
				expected: false
			}
		],
		(testCase) => {
			expect(pollDue(testCase.state, testCase.now, QUOTA_POLL_MS, testCase.visibility)).toBe(
				testCase.expected
			);
		}
	);
});

describe('clampPage', () => {
	forEachCase(
		[
			{
				name: 'keeps a page the data still covers',
				page: 2,
				total: 12,
				expected: 2
			},
			{
				name: 'parks a stranded page on the last page there is',
				page: 3,
				total: 1,
				expected: 1
			},
			{
				name: 'parks on the first page when the data is gone',
				page: 4,
				total: 0,
				expected: 1
			},
			{
				name: 'keeps the last page itself on an exact boundary',
				page: 2,
				total: 10,
				expected: 2
			}
		],
		(testCase) => {
			expect(clampPage(testCase.page, testCase.total, QUOTA_PAGE_SIZE)).toBe(testCase.expected);
		}
	);
});
