// The Overview's two bar charts and the reads they need (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 016 F1,
// F2, F4).
//
// The read is what this file is for: the panel's summary answers for one dimension per call, so the pair
// reads the two dimensions it draws and reuses the tab's own response when the operator's breakdown is one
// of them. The second claim is that one chart's refusal is not the other's, because each card renders its
// own read's state.
//
// The pair is rendered on its own rather than through the tab, and the stub answers by dimension
// (`tests/support/usage-overview-stub.ts`), which is what lets a row here say which read it means.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewBarCharts from '../../src/lib/components/UsageOverviewBarCharts.svelte';
import { providerSummaryBody, summaryBody } from '../support/usage-fixtures';
import { stubUsage } from '../support/usage-overview-stub';
import { summaryQueries } from '../support/usage-stub-queries';
import type { UsageBreakdown, UsageGroup, UsageTotals } from '$lib/schemas/usage';

const FROM = '2026-09-17T00:00:00Z';
const TO = '2026-09-18T00:00:00Z';

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

const NAMES = new Map([
	['openai', 'OpenAI'],
	['anthropic', 'Anthropic']
]);

/** The groups the tab already has when its breakdown is the default one, by model. */
const MODELS = [group('gpt-4o', { requests: 80, tokens_in: 2500, tokens_out: 900 })];

function draw(overrides: { groupBy?: UsageBreakdown; groups?: UsageGroup[] } = {}): void {
	render(UsageOverviewBarCharts, {
		props: {
			from: FROM,
			to: TO,
			granularity: 'hour',
			groupBy: overrides.groupBy ?? 'model',
			groups: overrides.groups ?? MODELS,
			providerNames: NAMES
		}
	});
}

/** One card, told apart from the other by its title. */
function card(title: string): HTMLElement {
	return screen.getByText(title).closest('figure') as HTMLElement;
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('UsageOverviewBarCharts', () => {
	it('reads the dimension the breakdown is not showing, and reuses the one it is', async () => {
		const stub = stubUsage();
		draw();

		expect(await screen.findByText('By provider')).toBeTruthy();

		// The provider chart's own read, for the window the tab read rather than one of its own.
		const reads = summaryQueries(stub, 'provider');
		expect(reads).toHaveLength(1);
		expect(reads[0].get('from')).toBe(FROM);
		expect(reads[0].get('to')).toBe(TO);

		// The model chart drew the groups the tab already had: no second read for the dimension on screen.
		expect(summaryQueries(stub, 'model')).toHaveLength(0);

		expect(
			within(card('By provider')).getByText(
				'OpenAI leads with 4.1K tokens, across the 2 providers with usage.'
			)
		).toBeTruthy();
		expect(
			within(card('Top models')).getByText(
				'gpt-4o leads with 3.4K tokens, across the 1 model with usage.'
			)
		).toBeTruthy();
	});

	it('draws the dimension the breakdown is showing without reading it again', async () => {
		const stub = stubUsage();
		draw({
			groupBy: 'provider',
			groups: [group('openai', { requests: 5, tokens_in: 4000, tokens_out: 4000 })]
		});

		expect(await screen.findByText('By provider')).toBeTruthy();

		expect(summaryQueries(stub, 'provider')).toHaveLength(0);
		expect(summaryQueries(stub, 'model')).toHaveLength(1);
		expect(
			within(card('By provider')).getByText(
				'OpenAI leads with 8K tokens, across the 1 provider with usage.'
			)
		).toBeTruthy();
	});

	it('keeps the other chart drawing when one read is refused', async () => {
		stubUsage({ providerSummaryStatus: 500 });
		draw();

		expect(await screen.findByText('By provider')).toBeTruthy();

		expect(
			within(card('By provider')).getByText(
				'The provider breakdown could not be read (the provider breakdown is unreachable).'
			)
		).toBeTruthy();
		expect(within(card('Top models')).getByText(/gpt-4o leads with 3.4K tokens/)).toBeTruthy();
	});

	it('says a chart has nothing to draw in the chosen measure', async () => {
		stubUsage({ providerSummary: providerSummaryBody({ groups: [] }) });
		draw();

		expect(await screen.findByText('By provider')).toBeTruthy();

		expect(
			within(card('By provider')).getByText('No provider has tokens in this window.')
		).toBeTruthy();
		expect(within(card('Top models')).getByText(/gpt-4o leads with/)).toBeTruthy();
	});

	it("switches one chart's measure without moving the other", async () => {
		stubUsage();
		draw();

		expect(await screen.findByText('By provider')).toBeTruthy();

		await fireEvent.click(within(card('By provider')).getByRole('button', { name: 'Requests' }));

		expect(
			within(card('By provider')).getByText(
				'OpenAI leads with 90 requests, across the 2 providers with usage.'
			)
		).toBeTruthy();
		// The other card's switch is its own state, so it is still ranking by tokens.
		expect(
			within(card('Top models'))
				.getByRole('button', { name: 'Tokens' })
				.getAttribute('aria-pressed')
		).toBe('true');
	});

	it('draws the top five models and says how many were left out', async () => {
		const seven = Array.from({ length: 7 }, (_, index) =>
			group(`model-${index}`, { requests: 7 - index, tokens_in: 700 - index * 100 })
		);
		// The tab's dimension is declared in the `summary` fixture because this row is about the model
		// chart's own read, and the stub routes a read by the dimension its fixture was written for.
		stubUsage({
			summary: summaryBody({ group_by: 'provider', groups: [group('openai')] }),
			modelSummary: summaryBody({ group_by: 'model', groups: seven })
		});
		draw({ groupBy: 'provider', groups: [group('openai', { tokens_in: 10 })] });

		expect(await screen.findByText('Top models')).toBeTruthy();

		const figure = card('Top models');
		expect(
			within(figure).getByText(
				'model-0 leads with 700 tokens; the top 5 of 7 models with usage are drawn.'
			)
		).toBeTruthy();
		// The tail is not drawn, but the table carries the groups that were, which is what the sentence counts.
		expect(within(figure).getByRole('table').querySelectorAll('tbody tr')).toHaveLength(5);
	});
});
