// The Overview's bar chart card (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 016 F1 and F2).
//
// The card is rendered on its own: the pair's own file measures the reads, and this file is about what one
// card states. Three claims matter more than the markup. A bar's length is relative to the longest bar and
// never zero for a group that has usage. The numbers behind the bars are one disclosure away, because the
// labels are rounded. And the bars are hidden from assistive technology, because the sentence and the table
// say the same thing.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageBarChart from '../../src/lib/components/UsageBarChart.svelte';
import { usageBars, type UsageMeasure } from '$lib/schemas/usage-bars';
import type { UsageGroup, UsageTotals } from '$lib/schemas/usage';

function group(key: string, overrides: Partial<UsageTotals> = {}): UsageGroup {
	return {
		key,
		totals: {
			requests: 10,
			tokens_in: 1000,
			tokens_out: 0,
			tokens_cache_read: 0,
			tokens_cache_write: 0,
			cost_usd: '0.0010',
			latency_ms: 0,
			latency_p50_ms: 0,
			latency_p95_ms: 0,
			error_count: 0,
			error_rate: '0.0000',
			...overrides
		}
	};
}

const GROUPS = [
	group('openai', { requests: 40, tokens_in: 3000, tokens_out: 1000 }),
	group('anthropic', { requests: 10, tokens_in: 800, tokens_out: 200 })
];

type Options = { measure?: UsageMeasure; groups?: UsageGroup[]; error?: string | null };

function draw(overrides: Options = {}): { onmeasure: ReturnType<typeof vi.fn> } {
	const measure = overrides.measure ?? 'tokens';
	const onmeasure = vi.fn();

	render(UsageBarChart, {
		props: {
			title: 'By provider',
			caption: 'Usage per provider in this window, largest first.',
			noun: 'provider',
			set: usageBars(overrides.groups ?? GROUPS, { measure, label: (key) => key }),
			measure,
			onmeasure,
			error: overrides.error ?? null
		}
	});

	return { onmeasure };
}

/** The card, told apart from the screen's other figures by its title. */
function card(): HTMLElement {
	return screen.getByText('By provider').closest('figure') as HTMLElement;
}

/**
 * The bars, which are hidden from assistive technology because the sentence and the table say the same
 * thing. Scoped because every bar's name and figure appears in that table too.
 */
function bars(): HTMLElement {
	return card().querySelector('[aria-hidden="true"]') as HTMLElement;
}

afterEach(() => {
	cleanup();
});

describe('UsageBarChart', () => {
	it('draws one bar per group, the longest at full width, and labels each with a compact figure', () => {
		draw();

		expect(within(bars()).getByText('openai')).toBeTruthy();
		expect(within(bars()).getByText('anthropic')).toBeTruthy();

		// 4,000 tokens against 1,000: the shorter bar is a quarter of the track rather than a second scale.
		const widths = Array.from(bars().querySelectorAll('span[style]')).map(
			(bar) => (bar as HTMLElement).style.width
		);
		expect(widths).toEqual(['100%', '25%']);
		expect(within(bars()).getByText('4K')).toBeTruthy();
		expect(within(bars()).getByText('1K')).toBeTruthy();
	});

	it('states the ranking in words and keeps the exact figures one disclosure away', () => {
		draw();

		const figure = card();
		expect(
			within(figure).getByText('openai leads with 4K tokens, across the 2 providers with usage.')
		).toBeTruthy();

		// The bar labels are rounded, so the table behind the disclosure is what carries the API's figure.
		const details = within(figure)
			.getByText('Show these numbers as a table')
			.closest('details') as HTMLElement;
		const table = within(details).getByRole('table');

		expect(within(table).getByRole('columnheader', { name: 'Provider' })).toBeTruthy();
		expect(within(table).getByText('4,000')).toBeTruthy();
		expect(within(table).getByText('40')).toBeTruthy();
	});

	it('keeps a name the column truncates whole in the markup and in its title', () => {
		draw({ groups: [group('a-model-name-long-enough-for-the-column-to-cut')] });

		const label = within(bars()).getByText('a-model-name-long-enough-for-the-column-to-cut');

		expect(label.getAttribute('title')).toBe('a-model-name-long-enough-for-the-column-to-cut');
		expect(label.className).toContain('truncate');
	});

	it('names the empty measure rather than drawing a group that did not happen', () => {
		draw({ measure: 'requests', groups: [group('openai', { requests: 0 })] });

		expect(screen.getByText('No provider has requests in this window.')).toBeTruthy();
		expect(screen.queryByText('Show these numbers as a table')).toBeNull();
	});

	it('replaces the bars with the read refusal, which is not an empty window', () => {
		draw({ error: 'The provider breakdown could not be read (the store is unreachable).' });

		expect(
			screen.getByText('The provider breakdown could not be read (the store is unreachable).')
		).toBeTruthy();
		// A chart that drew its bars under a refusal would state figures the read did not return.
		expect(screen.queryByText('openai')).toBeNull();
		expect(screen.queryByText('Show these numbers as a table')).toBeNull();
	});

	it('reports the measure the operator picked and leaves the choice to the caller', async () => {
		const { onmeasure } = draw();
		const figure = card();

		expect(
			within(figure).getByRole('button', { name: 'Tokens' }).getAttribute('aria-pressed')
		).toBe('true');
		expect(
			within(figure).getByRole('button', { name: 'Requests' }).getAttribute('aria-pressed')
		).toBe('false');

		await fireEvent.click(within(figure).getByRole('button', { name: 'Requests' }));

		expect(onmeasure).toHaveBeenCalledWith('requests');
		// The card holds no state of its own: the pressed button follows the prop, not the click.
		expect(
			within(figure).getByRole('button', { name: 'Tokens' }).getAttribute('aria-pressed')
		).toBe('true');
	});
});
