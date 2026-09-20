// Shared fake API for the Quota Tracker's budget caps (docs/SPEC-UI/001-SPEC-UI.md §6.6, U2).
//
// The stub stores caps and applies every write to them the way the server does, because the rule this
// slice turns on is a rule about state: the route REPLACES the whole cap set, so an amount the body omits
// clears that cap. A stub that echoed the request back would let a panel that sent a stale amount pass,
// and that panel would clear a budget in production. It also answers the two shapes the API distinguishes
// rather than one: an endpoint with nothing stored reports `cap: null`, while a cap that was cleared
// reports an object whose amounts are absent, which is what the Go response's `omitempty` pointers do.
//
// An endpoint the registry does not carry is refused with the server's own sentence and status, so the
// panel's handling of that answer is exercised rather than assumed.

import { vi } from 'vitest';

export type StoredCap = {
	monthly_cost_usd: string | null;
	monthly_tokens: number | null;
	updated_at: string;
};

/** The instant every write in this stub stores, so a test can tell the stored time from the read time. */
export const CAP_WRITTEN_AT = '2026-09-20T12:00:00Z';

// `domain.Decimal.String()` prints 8 places, so a stored amount always reads back with them.
function normalizeCost(value: string): string {
	const [whole, fraction = ''] = value.split('.');
	return `${whole}.${fraction.padEnd(8, '0').slice(0, 8)}`;
}

export type QuotaStub = {
	windows: Record<string, unknown>[];
	/** The endpoint list the screen reads for its labels, which is also what a cap write is checked against. */
	endpoints: Record<string, unknown>[];
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
				meta: { page: 1, per_page: 100, total: stub.endpoints.length }
			});
		}

		if (method === 'GET' && parsed.pathname.endsWith('/quotas')) {
			return json({ data: stub.windows });
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
			if (!stub.endpoints.some((endpoint) => endpoint.id === endpointId)) {
				return refusal('NOT_FOUND', 'upstream endpoint not found', 404);
			}

			return json(capResponse(endpointId, store(endpointId, body)));
		}

		return json({ error: { code: 'NOT_FOUND', message: 'no stub route' } }, 404);
	});

	return stub;
}
