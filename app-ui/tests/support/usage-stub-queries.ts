// Readers for the requests a Usage test's stub recorded (`tests/support/usage-overview-stub.ts`).
//
// They live apart from the stub because they answer a different question: the stub decides what the panel
// gets back, and these say what it asked for. The distinction matters on this screen more than most, since
// the tab's own summary read and the two bar charts' reads are the same route (draft 016 F4).

import { queryOf } from './page.svelte';
import type { UsageStub } from './usage-overview-stub';

/** The query of the last request the panel sent to one route. */
export function lastQuery(stub: UsageStub, route: string): URLSearchParams {
	const matches = stub.requested.filter((url) => url.includes(route));
	return new URLSearchParams(queryOf(matches[matches.length - 1] ?? ''));
}

/**
 * The queries of the summary reads that carried one `group_by`, or the reads that carried none when
 * `groupBy` is null.
 *
 * `lastQuery` no longer names one summary read: a test about the tab's read asks for the dimension the URL
 * chose, and a test about a chart's read asks for that dimension. Counting is what keeps the claim honest,
 * because the pair reuses the tab's response when the operator's dimension is one of the two it draws:
 * exactly one read carries that dimension.
 */
export function summaryQueries(stub: UsageStub, groupBy: string | null): URLSearchParams[] {
	return stub.requested
		.filter((url) => url.includes('/usage/summary'))
		.map((url) => new URLSearchParams(queryOf(url)))
		.filter((query) => query.get('group_by') === groupBy);
}

/**
 * The query of the last summary read that carried one dimension, or an empty query when the panel sent
 * none, so an assertion about the read fails rather than throwing on a missing one.
 */
export function lastSummaryQuery(stub: UsageStub, groupBy: string | null): URLSearchParams {
	const reads = summaryQueries(stub, groupBy);
	return reads[reads.length - 1] ?? new URLSearchParams();
}
