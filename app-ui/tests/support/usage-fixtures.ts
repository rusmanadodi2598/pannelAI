// The bodies the Usage Overview fixtures answer with (`tests/support/usage-overview-stub.ts`).
//
// They are data rather than behaviour, which is why they live apart from the fake API: the stub decides
// which of them answers a read, and these decide what a read says. Every builder takes an override bag so a
// row can change one field without restating the body.
//
// The summary fixture carries the model breakdown, because that is the tab's default `group_by` (draft 014
// F1) and a fixture that answered an empty group block would leave every test reading the empty state
// instead of the table.

import { provider } from './providers-route-stub';

export const HOUR_MS = 3_600_000;

export function totals(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		requests: 120,
		tokens_in: 4000,
		tokens_out: 1500,
		tokens_cache_read: 200,
		tokens_cache_write: 100,
		cost_usd: '0.0042',
		latency_ms: 2400,
		latency_p50_ms: 210,
		latency_p95_ms: 880,
		error_count: 15,
		error_rate: '0.1250',
		...overrides
	};
}

export function summaryBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		from: '2026-09-17T00:00:00Z',
		to: '2026-09-18T00:00:00Z',
		group_by: 'model',
		totals: totals(),
		// Two rows whose figures differ from the window's, so a test that looks for a tile's number cannot
		// match a breakdown cell instead.
		groups: [
			{
				key: 'gpt-4o',
				totals: totals({
					requests: 80,
					tokens_in: 2500,
					tokens_out: 900,
					cost_usd: '0.0026',
					error_count: 9,
					error_rate: '0.1125'
				})
			},
			{
				key: 'claude-sonnet-4',
				totals: totals({
					requests: 40,
					tokens_in: 1200,
					tokens_out: 500,
					cost_usd: '0.0016',
					error_count: 6,
					error_rate: '0.1500'
				})
			}
		],
		...overrides
	};
}

/**
 * The bar charts' provider-dimension answer (draft 016 F1).
 *
 * Its figures appear nowhere else on the screen: a chart cell that repeated a tile's number would make a
 * `getByText` assertion about the tile pass while it read the chart.
 */
export function providerSummaryBody(
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	return summaryBody({
		group_by: 'provider',
		groups: [
			{ key: 'openai', totals: totals({ requests: 90, tokens_in: 3000, tokens_out: 1100 }) },
			{ key: 'anthropic', totals: totals({ requests: 30, tokens_in: 1000, tokens_out: 400 }) }
		],
		...overrides
	});
}

export function providersBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		data: [provider(), provider({ id: 'anthropic', name: 'Anthropic', endpoint_count: 1 })],
		meta: { page: 1, per_page: 100, total: 2 },
		...overrides
	};
}

export function timeseriesBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		granularity: 'hour',
		from: '2026-09-17T00:00:00Z',
		to: '2026-09-18T00:00:00Z',
		buckets: [
			{ bucket: '2026-09-17T13:00:00Z', totals: totals({ requests: 100 }) },
			{ bucket: '2026-09-17T14:00:00Z', totals: totals({ requests: 20 }) }
		],
		...overrides
	};
}
