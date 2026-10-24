// The Quota Tracker's fixtures (docs/SPEC-UI/001-SPEC-UI.md §6.6): the wire rows a test writes, the shape
// of the stub's state, and the one line a card prints for a connection's counted windows.
//
// Split from `quota-stub.ts`, which is the fake API: this file is what a test *puts in*, that is what the
// fake *answers with*, and neither needs the other's rules.

export type StoredCap = {
	monthly_cost_usd: string | null;
	monthly_tokens: number | null;
	updated_at: string;
};

/** The instant every write in this stub stores, so a test can tell the stored time from the read time. */
export const CAP_WRITTEN_AT = '2026-09-20T12:00:00Z';

export type QuotaStub = {
	windows: Record<string, unknown>[];
	/** The endpoint list the screen reads for its labels, which is also what a cap write is checked against. */
	endpoints: Record<string, unknown>[] | null;
	/** The stored cap per endpoint. A missing key means no cap was ever stored for it. */
	caps: Record<string, StoredCap>;
	/** The endpoint ids the per-endpoint read was called with. */
	capReads: string[];
	/** Every write, with the body as it arrived, so a test can prove what was sent and what was left out. */
	capWrites: { endpointId: string; body: Record<string, unknown> }[];
	capReadStatus: number;
	capWriteRefusal: { status: number; code: string; message: string } | null;
	/** The endpoint list's own status, for a test that needs the label read to fail. */
	endpointStatus: number;
	/** The collection read's status, for a test that needs a poll to fail over data already on screen. */
	quotaStatus: number;
	/** Whether the collection read says its row ceiling cut the page. */
	truncated: boolean;
	/** Every paged /quotas read's URL, in order, so a test can prove what the screen asked for. */
	quotaReads: string[];
	/**
	 * When set, the paged read answers as if only this many provider groups existed, standing in for a
	 * poll that lands after the data shrank.
	 */
	shrinkTo: number | null;
	/**
	 * The published-quota payload per endpoint id. A missing key answers the server's own refusal, which
	 * is what an endpoint the gateway does not carry produces. The same payloads ride on the collection
	 * read for the endpoints that read's page names, because that is where the card gets its numbers now.
	 */
	published: Record<string, Record<string, unknown>>;
	/**
	 * When set, the collection read answers with the gateway's "the provider cache could not be read"
	 * sentence beside its own counts, and no `published` block at all.
	 */
	publishedNote: string | null;
	/** The endpoint ids the published read was called with, in order: the read is per card, not on load. */
	publishedReads: string[];
	/** The endpoint ids the published read was called with `force=1`, which is the operator's own press. */
	publishedForces: string[];
	/**
	 * When set, the published read's response is withheld until `releasePublished` runs. A screen that
	 * fired a second read while the first was still in flight is otherwise invisible to a test.
	 */
	holdPublished: boolean;
	/** Set once a held read has been asked for; the test calls it with the payload to answer with. */
	releasePublished: ((payload: Record<string, unknown>) => void) | null;
};

export function quotaWindowRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		endpoint_id: 'ep_1',
		provider_id: 'anthropic',
		window: 'monthly',
		used: 120000,
		limit: 200000,
		resets_at: new Date(Date.now() + 2 * 3_600_000).toISOString(),
		source: 'computed',
		...overrides
	};
}

export function quotaEndpointRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return { id: 'ep_1', label: 'Anthropic primary', ...overrides };
}

/** The instant a `never_polled` placeholder carries: a real timestamp that means no answer, not a gap. */
export const NEVER_POLLED_FETCHED_AT = '1970-01-01T00:00:00Z';

/**
 * A published-quota payload, shaped the way the API shapes one: amounts as decimal strings, a bucket
 * with no ceiling carrying no `total` key at all, and a soft outcome as a sentence beside an empty list.
 *
 * The reset is built from the wall clock the way `quotaWindowRow` is, so a countdown assertion stays
 * true whenever the suite runs.
 */
export function publishedUsage(
	endpointId: string,
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	return {
		endpoint_id: endpointId,
		provider_id: 'qoder',
		plan: 'personal_standard',
		fetched_at: '2026-09-28T12:00:00Z',
		data: [
			{
				label: 'Personal',
				used: '12.5',
				total: '3000',
				resets_at: new Date(Date.now() + 2 * 3_600_000).toISOString()
			}
		],
		...overrides
	};
}

/**
 * The `published[]` entry for an account the poll worker has not answered for yet, the windowless account
 * the reshape exists to make visible. It carries the placeholder `fetched_at`, empty `data`, and no message
 * or plan: the card's "Not polled yet" state is driven by the flag, not by an empty answer.
 */
export function neverPolledUsage(
	endpointId: string,
	providerId = 'qoder'
): Record<string, unknown> {
	return {
		endpoint_id: endpointId,
		provider_id: providerId,
		fetched_at: NEVER_POLLED_FETCHED_AT,
		data: [],
		never_polled: true
	};
}

/** The one line a connection's counted windows get, named by the window kind it picked out. */
export function quotaCountedLine(kind: string): RegExp {
	return new RegExp(`Counted by this gateway: .*· (most spent )?${kind}`);
}
