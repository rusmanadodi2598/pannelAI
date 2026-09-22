// The finished-requests list of the Usage live panel (draft 014 F4).
//
// Two rules are worth more than the markup here: the time beside a row is a duration rather than an
// instant, and it is kept fresh by a ticker, because a frozen "just now" would be a wrong figure a minute
// later. The second rule is what the fake clock measures, and the boundary it crosses is the minute.
//
// The component is rendered on its own rather than through the panel: the panel's own tests drive it from
// a frame (`usage-live-drawing.test.ts`), and this file is about the list's own claims: the elapsed time,
// its freshness, and the In/Out split (draft 016 F3), where a figure the frame never sent is stated as an
// absence rather than printed as a zero.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageRecentList from '../../src/lib/components/UsageRecentList.svelte';
import type { UsageLiveRecent } from '$lib/schemas/usage-live';
import { squashed } from '../support/dom';

const NOW = Date.parse('2026-09-22T12:00:00Z');

function recent(overrides: Partial<UsageLiveRecent> = {}): UsageLiveRecent {
	return {
		request_id: 'req_1',
		provider_id: 'openai',
		model: 'gpt-4o',
		ts: new Date(NOW - 2 * 3_600_000).toISOString(),
		status: 'success',
		...overrides
	};
}

function names(id: string): string {
	return id === 'openai' ? 'OpenAI' : id;
}

/** The list item carrying a row, read from the model cell so the split can be asserted with its words. */
function row(model = 'gpt-4o'): HTMLElement {
	const item = screen.getByText(model).closest('li');
	if (item === null) throw new Error('the model cell is not inside a list item');
	return item;
}

describe('UsageRecentList', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(NOW);
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it('dates each row as how long ago it finished', () => {
		render(UsageRecentList, {
			props: {
				recent: [
					recent(),
					recent({ request_id: 'req_2', ts: new Date(NOW - 30_000).toISOString() })
				],
				providerName: names
			}
		});

		expect(screen.getByText('2h ago')).toBeTruthy();
		expect(screen.getByText('just now')).toBeTruthy();
	});

	it('keeps the time fresh as the clock passes a boundary', async () => {
		render(UsageRecentList, {
			props: {
				recent: [recent({ ts: new Date(NOW - 59_500).toISOString() })],
				providerName: names
			}
		});

		expect(screen.getByText('just now')).toBeTruthy();

		await vi.advanceTimersByTimeAsync(1_000);

		// A frozen reading would still say "just now" here, which is the wrong figure this ticker exists to
		// prevent.
		expect(screen.getByText('1m ago')).toBeTruthy();
		expect(screen.queryByText('just now')).toBeNull();
	});

	it('states an absent time rather than leaving the cell blank', () => {
		render(UsageRecentList, {
			props: { recent: [recent({ ts: undefined })], providerName: names }
		});

		expect(screen.getByText('No time reported')).toBeTruthy();
	});

	it('splits the token counts a frame reported into in and out', () => {
		render(UsageRecentList, {
			props: {
				recent: [recent({ tokens_in: 12_345, tokens_out: 3_000 })],
				providerName: names
			}
		});

		expect(squashed(row())).toContain('12,345 in');
		expect(squashed(row())).toContain('3,000 out');
	});

	it('reports a zero the frame sent as a figure, and says so when it sent none', () => {
		render(UsageRecentList, {
			props: {
				recent: [
					recent({ request_id: 'req_1', tokens_in: 0, tokens_out: 1200 }),
					recent({ request_id: 'req_2', model: 'gpt-4o-mini' })
				],
				providerName: names
			}
		});

		// A zero that arrived is a measured figure, so it is printed as one; the same cell on a frame that
		// never carried the field is an absence, and the reference's `0` there (`UsageTable.js:8`) would be
		// a claim this list cannot make.
		expect(squashed(row())).toContain('0 in');
		expect(squashed(row())).toContain('1,200 out');
		expect(squashed(row('gpt-4o-mini'))).toContain('No token counts reported');
		expect(squashed(row('gpt-4o-mini'))).not.toMatch(/\d[\d,]* (in|out)/);
	});

	it('prints the one side of the split a half-reported frame carries', () => {
		render(UsageRecentList, {
			props: {
				recent: [recent({ tokens_in: 1_200, tokens_out: undefined })],
				providerName: names
			}
		});

		expect(squashed(row())).toContain('1,200 in');
		expect(squashed(row())).not.toMatch(/\d[\d,]* out/);
	});

	it('names the provider through the registry and labels the status', () => {
		render(UsageRecentList, {
			props: {
				recent: [
					recent({ status: 'error' }),
					recent({ request_id: 'req_2', provider_id: 'anthropic', model: undefined })
				],
				providerName: names
			}
		});

		expect(screen.getByText('OpenAI')).toBeTruthy();
		// An id the registry does not carry is shown as it arrived rather than dropped.
		expect(screen.getByText('anthropic')).toBeTruthy();
		expect(screen.getByText('No model reported')).toBeTruthy();
		expect(screen.getByText('Error')).toBeTruthy();
		expect(screen.getByText('Success')).toBeTruthy();
	});
});
