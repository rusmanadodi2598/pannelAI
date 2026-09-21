// Endpoint keys table tests (docs/SPEC-UI/001-SPEC-UI.md §6.2, §4).
//
// The rate-limit cell is the case here: §6.2 asks for a countdown, and the table used to print the RFC3339
// value the wire carries, which is a dump rather than a countdown. The expectation is built with the panel's
// own formatter, because `formatTimestamp` goes through `Intl.DateTimeFormat` and a literal date would assert
// the test runner's locale instead of the screen's behaviour.
//
// The clock is a prop, so the cases below move it rather than the fixture: a countdown that read `Date.now()`
// internally would render the same text for every row and every case.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { squashed } from '../support/dom';
import EndpointKeysTable from '../../src/lib/components/EndpointKeysTable.svelte';
import { endpointKeyRow } from '../support/endpoint-stub';
import { countdownText, formatTimestamp } from '$lib/utils/time';
import type { EndpointKey } from '$lib/schemas/endpoint';

const NOW = Date.parse('2026-09-21T12:00:00Z');

function keys(overrides: Record<string, unknown> = {}): EndpointKey[] {
	return [endpointKeyRow(overrides)] as EndpointKey[];
}

function renderTable(rows: EndpointKey[], now = NOW): void {
	render(EndpointKeysTable, {
		props: {
			keys: rows,
			authType: 'api_key',
			testing: null,
			now,
			ontest: () => {},
			onsettled: () => {},
			onremove: () => {}
		}
	});
}

function statusCell(): HTMLElement {
	return within(screen.getByRole('table')).getAllByRole('cell')[3];
}

describe('EndpointKeysTable rate limit', () => {
	afterEach(cleanup);

	forEachCase(
		[
			{ name: 'hours away', offsetMs: 2 * 3_600_000, expected: 'in 2h' },
			{ name: 'minutes away', offsetMs: 5 * 60_000, expected: 'in 5m' },
			{ name: 'under a minute away', offsetMs: 30_000, expected: 'in under a minute' },
			// The window's instant has passed but the worker has not refreshed the row yet: the countdown
			// states that rather than printing a negative duration.
			{ name: 'already past', offsetMs: -60_000, expected: 'any moment now' }
		],
		(testCase) => {
			const value = new Date(NOW + testCase.offsetMs).toISOString();
			renderTable(keys({ rate_limited_until: value }));

			const cell = squashed(statusCell());

			expect(cell).toContain(`rate limited until ${formatTimestamp(value)}`);
			expect(cell).toContain(`(${testCase.expected})`);
		}
	);

	it('renders the formatted instant, not the raw value the wire carried', () => {
		const value = new Date(NOW + 2 * 3_600_000).toISOString();
		renderTable(keys({ rate_limited_until: value }));

		const cell = squashed(statusCell());

		expect(cell).not.toContain(value);
		expect(cell).toContain(formatTimestamp(value));
	});

	it('says nothing about a rate limit on a key that has none', () => {
		renderTable(keys());

		expect(squashed(statusCell())).not.toContain('rate limited');
	});

	it('measures the countdown against the clock it was given, not the wall clock', () => {
		const value = new Date(NOW + 3_600_000).toISOString();
		// Half an hour behind the instant the fixture was built from, so the same row reads a longer wait.
		const clock = NOW - 30 * 60_000;
		renderTable(keys({ rate_limited_until: value }), clock);

		expect(squashed(statusCell())).toContain(`(${countdownText(value, clock)})`);
		expect(squashed(statusCell())).toContain('(in 1h 30m)');
	});
});
