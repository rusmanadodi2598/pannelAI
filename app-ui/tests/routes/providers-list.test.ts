// Provider registry list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The detail page had render tests and the list had none, so the screen an operator lands on first was the
// unverified one. What is checked here is what the screen decides: the category filter is one of the two
// filters the API accepts, the two empty states are different sentences because they mean different things,
// the refresh repeats the read the screen is showing rather than resetting it, and a failure offers the
// retry rather than a blank table.
//
// The search has its own file, `providers-search.test.ts`, because its contract is the server's: §6.3
// forbids filtering the registry in the browser, so what is typed travels as `?q`. The custom provider node
// set is a second read with its own state; those cases are in `providers-custom-section.test.ts`.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { squashed } from '../support/dom';
import { provider, registryQuery, stubProviders } from '../support/providers-route-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

describe('provider registry list', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders a row per provider, with the id the operator matches against a log line', async () => {
		stubProviders({ rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic' })] });

		render(ProvidersPage);

		const table = await screen.findByRole('table');
		expect(table.classList.contains('mobile-key-value-table')).toBe(true);
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

		await fireEvent.click(screen.getByRole('button', { name: 'media' }));

		await waitFor(() => expect(registryQuery(stub.queries).get('category')).toBe('media'));
		expect(registryQuery(stub.queries).get('page')).toBe('1');
		expect(await screen.findByText('Anthropic')).toBeTruthy();
		expect(screen.queryByText('OpenAI')).toBeNull();
	});

	it('says the registry itself is empty, without offering a filter to clear', async () => {
		stubProviders({ rows: [] });

		render(ProvidersPage);

		expect(await screen.findByText('The registry is empty')).toBeTruthy();
		expect(squashed(screen.getByText(/No providers registered yet/))).toContain(
			'No providers registered yet'
		);
		expect(screen.queryByRole('button', { name: 'Clear the filters' })).toBeNull();
	});

	it('names the filter that matched nothing, and clears both filters on request', async () => {
		const stub = stubProviders({ rows: [provider()] });
		render(ProvidersPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'media' }));
		expect(await screen.findByText('No provider matches this filter')).toBeTruthy();
		expect(squashed(screen.getByText(/narrow the registry/))).toContain('narrow the registry');

		const clear = screen.getByRole('button', { name: 'Clear the filters' });
		for (const token of ['min-h-11', 'inline-flex', 'items-center', 'underline']) {
			expect(clear.className, `clear-filters lost ${token}`).toContain(token);
		}

		const before = stub.queries.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Clear the filters' }));

		// Clearing drops both parameters rather than sending them empty, which is what the API reads as
		// "everything": the sentence covers both filters, so the action must too.
		await waitFor(() => expect(stub.queries.length).toBeGreaterThan(before));
		expect(registryQuery(stub.queries).has('category')).toBe(false);
		expect(registryQuery(stub.queries).has('q')).toBe(false);
		expect(await screen.findByText('OpenAI')).toBeTruthy();
	});

	it('re-reads the registry with the search still applied when the operator asks for it', async () => {
		const stub = stubProviders({
			rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic' })]
		});
		render(ProvidersPage);
		await screen.findByRole('table');

		await fireEvent.input(screen.getByLabelText('Search providers by name or id'), {
			target: { value: 'anthropic' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Search' }));
		await waitFor(() => expect(registryQuery(stub.queries).get('q')).toBe('anthropic'));

		const before = stub.queries.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		await waitFor(() => expect(stub.queries.length).toBeGreaterThan(before));
		// §8.6.2: the control repeats the read the screen is showing, search included.
		expect(registryQuery(stub.queries).get('q')).toBe('anthropic');
	});

	it('re-reads the registry with the filter still applied when the operator asks for it', async () => {
		const stub = stubProviders({
			rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic', category: 'media' })]
		});
		render(ProvidersPage);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'media' }));
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

		expect(await screen.findByText('Could not load the provider registry')).toBeTruthy();
		expect(screen.getByText('the registry is unavailable')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy();
		for (const token of ['min-h-11', 'inline-flex', 'items-center', 'underline']) {
			expect(
				screen.getByRole('button', { name: 'Try again' }).className,
				`retry lost ${token}`
			).toContain(token);
		}
		expect(screen.queryByRole('button', { name: 'Copy request ID' })).toBeNull();
	});

	it('shows and exposes a copy control for an INTERNAL_ERROR request ID', async () => {
		const cases = [
			{ requestId: 'req_01J8Y3A8C7M2' },
			{ requestId: 'req_01J8Y3A8C7M3' },
			{ requestId: 'req_01J8Y3A8C7M4' }
		];
		for (const { requestId } of cases) {
			cleanup();
			vi.unstubAllGlobals();
			stubProviders({ status: 500, requestId });

			render(ProvidersPage);

			expect(await screen.findByText(requestId)).toBeTruthy();
			expect(screen.getByRole('button', { name: 'Copy request ID' })).toBeTruthy();
		}
	});
});
