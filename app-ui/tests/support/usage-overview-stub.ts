// Fixture and fake API for the Usage Overview tests (docs/SPEC-UI/001-SPEC-UI.md §6.5).
//
// The screen makes four reads on mount: the summary, the timeseries, the registry (which resolves a
// provider key to its name for the breakdown table), and the live stream's route. The stub answers all four
// and records every request in order, because the assertions in the test files are about what the panel
// asked for rather than about what it rendered from the answer.
//
// The summary fixture carries the model breakdown, because that is the tab's default `group_by` (draft 014
// F1) and a fixture that answered an empty group block would leave every test reading the empty state
// instead of the table.
//
// The live route is answered 404 rather than left unhandled. That is what a gateway without the route
// answers (draft 012 F4), and it keeps this stub from mistaking the stream for one of the reads. The two
// halves are exercised together in `usage-overview-live.test.ts`.

import { vi } from 'vitest';
import { queryOf } from './page.svelte';
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

export type UsageStub = {
	/** Every request the panel made, in order, so a refresh can be told from a first read. */
	requested: string[];
	summary: Record<string, unknown>;
	timeseries: Record<string, unknown>;
	providers: Record<string, unknown>;
	/** The summary and timeseries refusal. The registry has its own, so one can fail without the other. */
	status: number;
	/** The registry read's own refusal, which leaves the table rendering provider ids. */
	providersStatus: number;
};

export function stubUsage(overrides: Partial<UsageStub> = {}): UsageStub {
	const stub: UsageStub = {
		requested: [],
		summary: summaryBody(),
		timeseries: timeseriesBody(),
		providers: providersBody(),
		status: 200,
		providersStatus: 200,
		...overrides
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		stub.requested.push(url);

		if (url.includes('/usage/live')) {
			return new Response('not found', { status: 404 });
		}

		// The registry is answered before the usage routes' fallback, so a failed aggregate read cannot
		// decide the names the table renders.
		if (url.includes('/providers')) {
			if (stub.providersStatus !== 200) {
				return new Response(
					JSON.stringify({
						error: { code: 'INTERNAL_ERROR', message: 'the registry is unreachable' }
					}),
					{ status: stub.providersStatus, headers: { 'content-type': 'application/json' } }
				);
			}

			return new Response(JSON.stringify(stub.providers), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});
		}

		const body = url.includes('/usage/timeseries') ? stub.timeseries : stub.summary;
		return new Response(JSON.stringify(body), {
			status: stub.status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

/** The query of the last request the panel sent to one route. */
export function lastQuery(stub: UsageStub, route: string): URLSearchParams {
	const matches = stub.requested.filter((url) => url.includes(route));
	return new URLSearchParams(queryOf(matches[matches.length - 1] ?? ''));
}
