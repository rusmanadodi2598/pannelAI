// Shared fake API for the Quota Tracker (docs/SPEC-UI/001-SPEC-UI.md §6.6, U2).
//
// The stub stores caps and applies every write to them the way the server does, because the rule this slice
// turns on is a rule about state: the route REPLACES the whole cap set, so an amount the body omits clears
// that cap. A stub that echoed the request back would let a panel that sent a stale amount pass, and that
// panel would clear a budget in production. It also answers the two shapes the API distinguishes rather than
// one: an endpoint with nothing stored reports `cap: null`, while a cap that was cleared reports an object
// whose amounts are absent, which is what the Go response's `omitempty` pointers do.
//
// An endpoint the registry does not carry is refused with the server's own sentence and status, so the
// panel's handling of that answer is exercised rather than assumed.
//
// The provider's answers are stubbed twice over, the way the route serves them now: the collection read
// carries the entries for the endpoints its own page names (which is where the card gets its numbers, with
// no read of its own), and the per-endpoint read stays a call a test can count. A stub that answered the
// per-endpoint route on every load would let a screen that fired one request per card pass, and that screen
// is the fan-out the collection block exists to remove. `publishedReads` and `publishedForces` are what a
// test measures those two paths against.
//
// The rows, the stored-cap shape and the stub's own state type are in `quota-fixtures.ts`.

import { vi } from 'vitest';
import { CAP_WRITTEN_AT, quotaEndpointRow, type QuotaStub, type StoredCap } from './quota-fixtures';

// `domain.Decimal.String()` prints 8 places, so a stored amount always reads back with them.
function normalizeCost(value: string): string {
	const [whole, fraction = ''] = value.split('.');
	return `${whole}.${fraction.padEnd(8, '0').slice(0, 8)}`;
}

export function stubQuota(overrides: Partial<QuotaStub> = {}): QuotaStub {
	const stub: QuotaStub = {
		windows: [],
		endpoints: [quotaEndpointRow()],
		caps: {},
		capReads: [],
		capWrites: [],
		capReadStatus: 200,
		capWriteRefusal: null,
		endpointStatus: 200,
		quotaReads: [],
		shrinkTo: null,
		published: {},
		publishedNote: null,
		publishedReads: [],
		publishedForces: [],
		holdPublished: false,
		releasePublished: null,
		...overrides
	};

	const json = (payload: unknown, status = 200): Response =>
		new Response(JSON.stringify(payload), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	const refusal = (code: string, message: string, status: number): Response =>
		json({ error: { code, message } }, status);

	// The Go response marks both amounts `omitempty`, so a cleared cap answers without the keys rather than
	// with nulls. An amount the body omits clears that cap: the route replaces the whole set.
	function capResponse(endpointId: string, cap: StoredCap): Record<string, unknown> {
		const payload: Record<string, unknown> = {
			endpoint_id: endpointId,
			updated_at: cap.updated_at
		};
		if (cap.monthly_cost_usd !== null) payload.monthly_cost_usd = cap.monthly_cost_usd;
		if (cap.monthly_tokens !== null) payload.monthly_tokens = cap.monthly_tokens;
		return payload;
	}

	function store(endpointId: string, body: Record<string, unknown>): StoredCap {
		const cost = body.monthly_cost_usd;
		const cap: StoredCap = {
			// The server parses the amount into a decimal and prints it with its 8 places, so a stored cap
			// reads back as `40.00000000` rather than as the string that was sent.
			monthly_cost_usd: typeof cost === 'string' ? normalizeCost(cost) : null,
			monthly_tokens: typeof body.monthly_tokens === 'number' ? body.monthly_tokens : null,
			updated_at: CAP_WRITTEN_AT
		};
		stub.caps[endpointId] = cap;
		return cap;
	}

	// The collection body the way SPEC-API §7.12 shapes it now: the page's windows, the meta block, and the
	// provider's answers for every account whose provider is on THIS page. A group is selected by the accounts
	// that exist, not the windows that happen to exist, so a `published[]` entry whose endpoint carries no
	// counted window still rides along: that entry is the windowless account the card has to show. An
	// endpoint the worker has not answered is present with `never_polled` rather than absent, which is the
	// "never polled" state the card has to render. `published_note` replaces the whole block when the cache
	// could not be read.
	function quotasBody(
		page: number,
		perPage: number,
		visible: Set<string>,
		totalGroups: number
	): Record<string, unknown> {
		const pageWindows = stub.windows.filter((window) => visible.has(String(window.provider_id)));

		const body: Record<string, unknown> = {
			data: pageWindows,
			meta: { page, per_page: perPage, total: totalGroups }
		};

		if (stub.publishedNote !== null) {
			body.published_note = stub.publishedNote;
			return body;
		}

		body.published = Object.entries(stub.published)
			.filter(([, payload]) => visible.has(String(payload.provider_id)))
			.map(([, payload]) => payload);
		return body;
	}

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		const url = String(input);
		const body: Record<string, unknown> =
			init?.body === undefined ? {} : JSON.parse(String(init.body));
		const parsed = new URL(url, 'http://panel.test');

		if (parsed.pathname.endsWith('/endpoints')) {
			if (stub.endpointStatus !== 200) {
				return refusal(
					'INTERNAL_ERROR',
					'The endpoint list could not be read.',
					stub.endpointStatus
				);
			}
			return json({
				data: stub.endpoints,
				meta: { page: 1, per_page: 100, total: stub.endpoints?.length ?? 0 }
			});
		}

		if (method === 'GET' && parsed.pathname.endsWith('/quotas')) {
			// The paged collection read (docs/PORT/006-PORT-QUOTA-PAGING.md D1), grouped by the accounts that
			// exist (the windowless-account reshape): a provider group is on the page when it has an account,
			// whether that account's only trace is a `published[]` entry with no counted window. Groups keep
			// first-seen order (windows first, then published-only accounts), and the meta block states the
			// total group count. The grouping here mirrors the SQL the server runs, which the integration test
			// proves against a real database.
			stub.quotaReads.push(url);
			const page = Number(parsed.searchParams.get('page') ?? '1');
			const perPage = Number(parsed.searchParams.get('per_page') ?? '25');

			const providers: string[] = [];
			for (const window of stub.windows) {
				const pid = String(window.provider_id);
				if (!providers.includes(pid)) providers.push(pid);
			}
			for (const payload of Object.values(stub.published)) {
				const pid = String(payload.provider_id);
				if (!providers.includes(pid)) providers.push(pid);
			}
			const kept = providers.slice(0, stub.shrinkTo ?? providers.length);
			const visible = new Set(kept.slice((page - 1) * perPage, page * perPage));

			return json(quotasBody(page, perPage, visible, kept.length));
		}

		const publishedMatch = /\/quotas\/([^/?]+)\/usage$/.exec(parsed.pathname);

		if (method === 'GET' && publishedMatch) {
			const endpointId = decodeURIComponent(publishedMatch[1]);
			stub.publishedReads.push(endpointId);
			// The route reads a literal "1" (§7.12's wantForce). A panel that sent `force=true` would be
			// served the cache while claiming to have asked the provider, so the flag is checked here.
			if (parsed.searchParams.get('force') === '1') stub.publishedForces.push(endpointId);

			const payload = stub.published[endpointId];
			if (stub.holdPublished) {
				return new Promise<Response>((resolve) => {
					stub.releasePublished = (held: Record<string, unknown>) => resolve(json(held));
				});
			}
			if (!payload) {
				// The server refuses an endpoint it does not carry before any provider call, and the
				// sentence is its own rather than one the panel writes.
				return refusal('NOT_FOUND', 'upstream endpoint not found', 404);
			}
			return json(payload);
		}

		const quotaMatch = /\/quotas\/([^/?]+)$/.exec(parsed.pathname);

		if (method === 'GET' && quotaMatch) {
			const endpointId = decodeURIComponent(quotaMatch[1]);
			stub.capReads.push(endpointId);
			if (stub.capReadStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The quota cap could not be read.', stub.capReadStatus);
			}

			const stored = stub.caps[endpointId];
			return json({
				endpoint_id: endpointId,
				// No `omitempty` on the Go side: nothing stored answers an explicit null, which the form
				// renders as two empty fields rather than as a cap of zero.
				cap: stored ? capResponse(endpointId, stored) : null,
				data: stub.windows.filter((window) => window.endpoint_id === endpointId)
			});
		}

		if (method === 'PUT' && quotaMatch) {
			const endpointId = decodeURIComponent(quotaMatch[1]);
			stub.capWrites.push({ endpointId, body });
			if (stub.capWriteRefusal) {
				const { status, code, message } = stub.capWriteRefusal;
				return refusal(code, message, status);
			}
			if (!stub.endpoints?.some((endpoint) => endpoint.id === endpointId)) {
				return refusal('NOT_FOUND', 'upstream endpoint not found', 404);
			}

			return json(capResponse(endpointId, store(endpointId, body)));
		}

		return json({ error: { code: 'NOT_FOUND', message: 'no stub route' } }, 404);
	});

	return stub;
}
