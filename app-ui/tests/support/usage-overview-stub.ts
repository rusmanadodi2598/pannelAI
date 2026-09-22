// Fixture and fake API for the Usage Overview tests (docs/SPEC-UI/001-SPEC-UI.md §6.5).
//
// The screen makes more than four reads on mount: the summary for the dimension the URL chose, the
// timeseries, the registry (which resolves a provider key to its name for the breakdown table), the bar
// charts' own dimension reads (draft 016 F1 and F2), and the live stream's route. The stub answers all of
// them and records every request in order, because the assertions in the test files are about what the
// panel asked for rather than about what it rendered from the answer.
//
// Three fixtures answer the summary route, because the tab and the charts read the same route for
// different dimensions. `summary` answers the tab's own read, and `providerSummary` and `modelSummary`
// answer the bar charts' reads. The stub tells them apart by the dimension each fixture was written for: a
// read carrying that dimension, or carrying no dimension at all (which is what the tab sends when the URL
// says there is no breakdown), is the tab's, and the charts only ever read the dimensions the tab is not
// showing. `tests/support/usage-stub-queries.ts` is how a test asks for one of those reads, since "the last
// summary request" no longer names one.
//
// The live route is answered 404 rather than left unhandled. That is what a gateway without the route
// answers (draft 012 F4), and it keeps this stub from mistaking the stream for one of the reads. The two
// halves are exercised together in `usage-overview-live.test.ts`.
//
// The bodies themselves live in `tests/support/usage-fixtures.ts`: this file decides which of them answers
// a read, and that file decides what a read says.

import { vi } from 'vitest';
import { queryOf } from './page.svelte';
import { providersBody, providerSummaryBody, summaryBody, timeseriesBody } from './usage-fixtures';

export type UsageStub = {
	/** Every request the panel made, in order, so a refresh can be told from a first read. */
	requested: string[];
	/** The tab's own summary read, whatever dimension the URL chose. */
	summary: Record<string, unknown>;
	/** The bar charts' provider-dimension read (draft 016 F1). */
	providerSummary: Record<string, unknown>;
	/** The bar charts' model-dimension read (draft 016 F2). */
	modelSummary: Record<string, unknown>;
	timeseries: Record<string, unknown>;
	providers: Record<string, unknown>;
	/** The summary and timeseries refusal. The registry has its own, so one can fail without the other. */
	status: number;
	/** The provider chart's own refusal, so one chart can fail while the other still draws. */
	providerSummaryStatus: number;
	/** The registry read's own refusal, which leaves the table rendering provider ids. */
	providersStatus: number;
};

/** The summary body for one read: the fixture written for that dimension, or the tab's own. */
function summaryBodyFor(url: string, stub: UsageStub): Record<string, unknown> {
	const dimension = new URLSearchParams(queryOf(url)).get('group_by');
	const own = String(stub.summary.group_by ?? '');

	if (dimension === null || dimension === own) return stub.summary;
	if (dimension === 'provider') return stub.providerSummary;
	if (dimension === 'model') return stub.modelSummary;

	// A dimension the stub has no chart fixture for is the tab's read, as it was before the charts existed.
	return stub.summary;
}

export function stubUsage(overrides: Partial<UsageStub> = {}): UsageStub {
	const stub: UsageStub = {
		requested: [],
		summary: summaryBody(),
		providerSummary: providerSummaryBody(),
		modelSummary: summaryBody(),
		timeseries: timeseriesBody(),
		providers: providersBody(),
		status: 200,
		providerSummaryStatus: 200,
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

		if (url.includes('/usage/timeseries')) {
			return new Response(JSON.stringify(stub.timeseries), {
				status: stub.status,
				headers: { 'content-type': 'application/json' }
			});
		}

		const providerChart =
			new URLSearchParams(queryOf(url)).get('group_by') === 'provider' &&
			String(stub.summary.group_by ?? '') !== 'provider';

		if (providerChart && stub.providerSummaryStatus !== 200) {
			return new Response(
				JSON.stringify({
					error: { code: 'INTERNAL_ERROR', message: 'the provider breakdown is unreachable' }
				}),
				{ status: stub.providerSummaryStatus, headers: { 'content-type': 'application/json' } }
			);
		}

		return new Response(JSON.stringify(summaryBodyFor(url, stub)), {
			status: stub.status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}
