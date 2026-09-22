// Fixture and fake API for the Usage Overview tests (docs/SPEC-UI/001-SPEC-UI.md §6.5).
//
// The screen makes three reads on mount: the summary, the timeseries, and the live stream's route. The stub
// answers all three and records every request in order, because the assertions in the test files are about
// what the panel asked for rather than about what it rendered from the answer.
//
// The live route is answered 404 rather than left unhandled. That is what a gateway without the route
// answers (draft 012 F4), and it keeps this stub from mistaking the stream for one of the two reads. The
// two halves are exercised together in `usage-overview-live.test.ts`.

import { vi } from 'vitest';
import { queryOf } from './page.svelte';

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
		group_by: '',
		totals: totals(),
		groups: [],
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
	status: number;
};

export function stubUsage(overrides: Partial<UsageStub> = {}): UsageStub {
	const stub: UsageStub = {
		requested: [],
		summary: summaryBody(),
		timeseries: timeseriesBody(),
		status: 200,
		...overrides
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		stub.requested.push(url);

		if (url.includes('/usage/live')) {
			return new Response('not found', { status: 404 });
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
