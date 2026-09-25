// Shared fixture and fake API for the `/providers` route tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The screen reads twice: the registry, which is paged and filtered, and the custom provider node set,
// which is neither (§7.4). One stub answers both paths and records every request, so a test can tell the
// two reads apart instead of reading "the last request" and hoping it was the one it meant.
//
// The registry read is answered with the server's paging envelope and filters by category the way the API
// does, so a screen that filtered in the browser would send no parameter and fail the cases that check the
// query. The node read is answered with §7.4's envelope, which carries no meta block.

import { vi } from 'vitest';

export function provider(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'chat',
		auth_type: 'api_key',
		auth_modes: ['api_key'],
		has_oauth: false,
		no_auth: false,
		routability: 'routable',
		endpoint_count: 2,
		status_summary: { total: 2, active: 2, disabled: 0, error: 0, rate_limited: 0 },
		...overrides
	};
}

export type StubOptions = {
	rows?: Record<string, unknown>[];
	total?: number;
	/** The registry read's own refusal. The node read has its own, so one can fail without the other. */
	status?: number;
	message?: string;
	nodes?: Record<string, unknown>[];
	nodesStatus?: number;
};

export type ProvidersStub = {
	/** Every request the page made, in order, so a refresh can be told from a re-render. */
	queries: string[];
	options: StubOptions;
};

/** Answers both reads the screen makes, and records every request it saw. */
export function stubProviders(options: StubOptions = {}): ProvidersStub {
	const queries: string[] = [];
	const rows = options.rows ?? [provider()];

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		queries.push(url);

		// The node read is answered first, so `status` above stays the registry read's refusal: a node read
		// that failed beside it would put a second "Try again" on the screen and the retry cases would stop
		// telling the two apart. The paths cannot be confused either, since `/provider-nodes` does not
		// contain `/providers`.
		if (url.includes('/provider-nodes')) {
			if (options.nodesStatus && options.nodesStatus !== 200) {
				return new Response(
					JSON.stringify({
						error: { code: 'INTERNAL_ERROR', message: 'the node store is unreachable' }
					}),
					{ status: options.nodesStatus, headers: { 'content-type': 'application/json' } }
				);
			}

			// §7.4 answers the whole set with no paging envelope, which is what the section parses.
			return new Response(JSON.stringify({ data: options.nodes ?? [] }), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});
		}

		if (options.status && options.status !== 200) {
			return new Response(
				JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: options.message ?? 'boom' } }),
				{ status: options.status, headers: { 'content-type': 'application/json' } }
			);
		}

		const params = new URL(url, 'http://panel.test').searchParams;
		const category = params.get('category');
		const query = params.get('q');
		// Both filters narrow the way the API does: category by equality, the search term as a
		// case-insensitive substring of the id or the name (PORT 002 D1). A screen that filtered in the
		// browser would send no parameter and fail the cases that check the query.
		const matching = rows.filter((row) => {
			if (category && row.category !== category) return false;
			if (query) {
				const needle = query.toLowerCase();
				const id = String(row.id ?? '');
				const name = String(row.name ?? '');
				if (!id.toLowerCase().includes(needle) && !name.toLowerCase().includes(needle)) {
					return false;
				}
			}
			return true;
		});

		return new Response(
			JSON.stringify({
				data: matching,
				meta: { page: 1, per_page: 25, total: options.total ?? matching.length }
			}),
			{ status: 200, headers: { 'content-type': 'application/json' } }
		);
	});

	return { queries, options };
}

/** Every request the page made to one path, so one read's refresh can be told from the other's. */
export function queriesTo(queries: string[], path: string): string[] {
	return queries.filter((url) => new URL(url, 'http://panel.test').pathname === path);
}

/**
 * The registry read, told apart from the node read and the paging control's own request by its path.
 *
 * The screen reads both paths on mount, so "the last request" is not the registry's by construction; the
 * filter is what keeps this helper meaning what its name says.
 */
export function registryQuery(queries: string[]): URLSearchParams {
	const last = queriesTo(queries, '/api/v1/providers').at(-1) ?? '';
	return new URL(last, 'http://panel.test').searchParams;
}
