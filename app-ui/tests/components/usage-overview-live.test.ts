// The two halves of the Usage tab together (draft 012 F2).
//
// The rule this file exists for is a negative one: the stream must not be able to change a figure the REST
// reads returned. It is proved by giving a frame every aggregate-shaped field a gateway might send and
// asserting that the tiles still show what the summary said. The structural half of the proof is in
// `usage-live-view.test.ts`, where the live state has no field an aggregate could be written into; this is
// the same claim measured on the rendered screen, because a type can be right while a screen is wrong.
//
// The second row is the placement rule: the live half sits outside the aggregate read's branches, so a
// gateway that cannot answer the summary still shows what it is routing right now.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import UsageOverviewTab from '../../src/lib/components/UsageOverviewTab.svelte';
import { frameText, liveStreams, type LiveBody } from '../support/live-stream';
import { provider } from '../support/providers-route-stub';
import { visit } from '../support/page.svelte';

vi.mock('$app/state', async () => {
	const { pageState: page } = await import('../support/page.svelte');
	return { page };
});

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

vi.mock('$app/navigation', async () => {
	const { pageState: page, queryOf: query } = await import('../support/page.svelte');
	return {
		goto: (url: string) => {
			page.url = { pathname: '/usage', searchParams: new SvelteURLSearchParams(query(url)) };
			return Promise.resolve();
		}
	};
});

const REST_REQUESTS = 120;
const FRAME_REQUESTS = 999999;

function totals(requests: number): Record<string, unknown> {
	return {
		requests,
		tokens_in: 4000,
		tokens_out: 1500,
		tokens_cache_read: 200,
		tokens_cache_write: 100,
		cost_usd: '0.0042',
		latency_ms: 2400,
		latency_p50_ms: 210,
		latency_p95_ms: 880,
		error_count: 15,
		error_rate: '0.1250'
	};
}

function stubTab(options: { usageStatus?: number } = {}): { streams: LiveBody[] } {
	const { answer, streams } = liveStreams();

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);

		if (url.includes('/usage/live')) return answer();

		if (url.includes('/providers')) {
			return new Response(
				JSON.stringify({
					data: [provider({ id: 'openai', name: 'OpenAI', endpoint_count: 2 })],
					meta: { page: 1, per_page: 100, total: 1 }
				}),
				{ status: 200, headers: { 'content-type': 'application/json' } }
			);
		}

		if (options.usageStatus && options.usageStatus !== 200) {
			return new Response(
				JSON.stringify({
					error: { code: 'INTERNAL_ERROR', message: 'the usage store is unreachable' }
				}),
				{ status: options.usageStatus, headers: { 'content-type': 'application/json' } }
			);
		}

		const body = url.includes('/usage/timeseries')
			? {
					granularity: 'hour',
					from: '2026-09-21T00:00:00Z',
					to: '2026-09-22T00:00:00Z',
					buckets: [{ bucket: '2026-09-21T13:00:00Z', totals: totals(REST_REQUESTS) }]
				}
			: {
					from: '2026-09-21T00:00:00Z',
					to: '2026-09-22T00:00:00Z',
					group_by: 'model',
					totals: totals(REST_REQUESTS),
					groups: [{ key: 'gpt-4o', totals: totals(REST_REQUESTS) }]
				};

		return new Response(JSON.stringify(body), {
			status: 200,
			headers: { 'content-type': 'application/json' }
		});
	});

	return { streams };
}

/** A frame carrying the live fields plus every aggregate-shaped field a gateway might send beside them. */
function greedyFrame(): string {
	return frameText({
		active: [],
		recent: [],
		error_provider: '',
		requests: FRAME_REQUESTS,
		total: FRAME_REQUESTS,
		totals: totals(FRAME_REQUESTS),
		summary: { totals: totals(FRAME_REQUESTS) },
		buckets: [{ bucket: '2026-09-22T00:00:00Z', totals: totals(FRAME_REQUESTS) }]
	});
}

beforeEach(() => {
	visit('/usage');
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('UsageOverviewTab with the live half', () => {
	it('keeps the REST figures when a frame carries figures of its own', async () => {
		const stub = stubTab();
		render(UsageOverviewTab);

		// More than one element shows it (the tile and the chart's own table), so this counts rather than
		// names one, and the claim is about all of them.
		expect((await screen.findAllByText(String(REST_REQUESTS))).length).toBeGreaterThan(0);

		stub.streams[0].send(greedyFrame());
		expect(await screen.findByText('Live')).toBeTruthy();

		expect(screen.getAllByText(String(REST_REQUESTS)).length).toBeGreaterThan(0);
		expect(screen.queryAllByText(String(FRAME_REQUESTS))).toHaveLength(0);
		expect(screen.queryAllByText('999,999')).toHaveLength(0);
	});

	it('keeps the live half on screen when the aggregate read fails', async () => {
		const stub = stubTab({ usageStatus: 500 });
		render(UsageOverviewTab);

		expect(await screen.findByText('Usage could not be loaded')).toBeTruthy();

		stub.streams[0].send(greedyFrame());

		expect(await screen.findByText('Live')).toBeTruthy();
		expect(screen.getByText(/Reading the gateway live stream\./)).toBeTruthy();
	});

	it('draws the configured provider and lights it when the stream says it is routing', async () => {
		const stub = stubTab();
		const container = render(UsageOverviewTab).container;

		expect(await screen.findByText('OpenAI')).toBeTruthy();
		expect(container.querySelector('.animate-ping')).toBeNull();

		stub.streams[0].send(
			frameText({
				active: [{ provider_id: 'openai', model: 'gpt-4o', started_at: new Date().toISOString() }],
				recent: [],
				error_provider: ''
			})
		);

		expect(await screen.findByText('OpenAI (gpt-4o).')).toBeTruthy();
		expect(container.querySelector('.animate-ping')).toBeTruthy();
	});
});
