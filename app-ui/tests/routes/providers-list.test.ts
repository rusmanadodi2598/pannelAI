// Provider registry list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The detail page had render tests and the list had none, so the screen an operator lands on first was the
// unverified one. What is checked here is what the screen decides: the category filter is the one filter
// §6.3 offers and the API accepts (a search over name and id has no parameter yet, so the control is
// absent rather than fake), the two empty states are different sentences because they mean different
// things, and a failure offers the retry rather than a blank table.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { squashed } from '../support/dom';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function provider(overrides: Record<string, unknown> = {}): Record<string, unknown> {
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

type StubOptions = {
	rows?: Record<string, unknown>[];
	total?: number;
	status?: number;
	message?: string;
};

/** Answers the registry read, filters by category the way the API does, and records every query it saw. */
function stubProviders(options: StubOptions = {}): { queries: string[] } {
	const queries: string[] = [];
	const rows = options.rows ?? [provider()];

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		queries.push(url);

		if (options.status && options.status !== 200) {
			return new Response(
				JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: options.message ?? 'boom' } }),
				{ status: options.status, headers: { 'content-type': 'application/json' } }
			);
		}

		const category = new URL(url, 'http://panel.test').searchParams.get('category');
		const matching = category ? rows.filter((row) => row.category === category) : rows;

		return new Response(
			JSON.stringify({
				data: matching,
				meta: { page: 1, per_page: 25, total: options.total ?? matching.length }
			}),
			{ status: 200, headers: { 'content-type': 'application/json' } }
		);
	});

	return { queries };
}

/** The registry read, told apart from the paging control's own request by its path alone. */
function registryQuery(queries: string[]): URLSearchParams {
	const last = queries.at(-1) ?? '';
	return new URL(last, 'http://panel.test').searchParams;
}

describe('provider registry list', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders a row per provider, with the id the operator matches against a log line', async () => {
		stubProviders({ rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic' })] });

		render(ProvidersPage);

		expect(await screen.findByRole('table')).toBeTruthy();
		expect(screen.getByText('OpenAI')).toBeTruthy();
		expect(screen.getByText('openai')).toBeTruthy();
		expect(screen.getByText('Anthropic')).toBeTruthy();
		// The status column states the summary rather than four numbers (statusSummaryText).
		expect(screen.getAllByText('2 active')).toHaveLength(2);
	});

	it('sends the category the operator picked, and returns to the first page to do it', async () => {
		const stub = stubProviders({
			rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic', category: 'media' })]
		});
		render(ProvidersPage);
		await screen.findByRole('table');

		await fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'media' } });

		await waitFor(() => expect(registryQuery(stub.queries).get('category')).toBe('media'));
		expect(registryQuery(stub.queries).get('page')).toBe('1');
		expect(await screen.findByText('Anthropic')).toBeTruthy();
		expect(screen.queryByText('OpenAI')).toBeNull();
	});

	it('says the registry itself is empty, without offering a filter to clear', async () => {
		stubProviders({ rows: [] });

		render(ProvidersPage);

		expect(await screen.findByText('The registry is empty')).toBeTruthy();
		expect(squashed(screen.getByText(/built without one/))).toContain('built without one');
		expect(screen.queryByRole('button', { name: 'Clear the filter' })).toBeNull();
	});

	it('names the category that matched nothing, and clears it on request', async () => {
		const stub = stubProviders({ rows: [provider()] });
		render(ProvidersPage);
		await screen.findByRole('table');

		await fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'media' } });

		expect(await screen.findByText('No provider in the media category')).toBeTruthy();

		const before = stub.queries.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Clear the filter' }));

		// Clearing drops the parameter rather than sending it empty, which is what the API reads as "all".
		await waitFor(() => expect(stub.queries.length).toBeGreaterThan(before));
		expect(registryQuery(stub.queries).has('category')).toBe(false);
		expect(await screen.findByText('OpenAI')).toBeTruthy();
	});

	it('re-reads the registry with the filter still applied when the operator asks for it', async () => {
		const stub = stubProviders({
			rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic', category: 'media' })]
		});
		render(ProvidersPage);
		await screen.findByRole('table');

		await fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'media' } });
		await waitFor(() => expect(registryQuery(stub.queries).get('category')).toBe('media'));

		const before = stub.queries.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		await waitFor(() => expect(stub.queries.length).toBeGreaterThan(before));
		// §8.6.2: the control repeats the read the screen is showing, rather than resetting it to the whole
		// registry, which would silently undo the filter the operator set.
		expect(registryQuery(stub.queries).get('category')).toBe('media');
	});

	it('reports a failed read and retries it on request', async () => {
		stubProviders({ status: 500, message: 'the registry is unavailable' });

		render(ProvidersPage);

		expect(await screen.findByText('The provider registry could not be loaded')).toBeTruthy();
		expect(screen.getByText('the registry is unavailable')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy();
	});
});
