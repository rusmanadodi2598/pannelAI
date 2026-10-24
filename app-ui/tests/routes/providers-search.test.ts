// The provider registry's own search (docs/SPEC-UI/001-SPEC-UI.md §6.3, PORT 002 D1).
//
// The search is the one control on this screen whose contract is a server's: §6.3's pagination discipline
// forbids filtering the registry in the browser, so what the operator types travels as `?q` and comes back
// as a page of rows. The bound is the API's too, measured in code points rather than UTF-16 units, which is
// why the two limit rows below run astral text: a length rule counted in the wrong unit refuses a value the
// server accepts, and the operator cannot see the difference.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { provider, registryQuery, stubProviders } from '../support/providers-route-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

/** The registry reads, told apart from the node read the screen also makes on mount. */
function registryReads(queries: string[]): string[] {
	return queries.filter((url) => new URL(url, 'http://panel.test').pathname.endsWith('/providers'));
}

describe('provider registry search', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('sends the search the operator typed, trimmed, and returns to the first page to do it', async () => {
		const stub = stubProviders({
			rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic' })]
		});
		render(ProvidersPage);
		await screen.findByRole('table');

		const input = screen.getByLabelText('Search providers by name or id');
		await fireEvent.input(input, { target: { value: '  anthropic  ' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Search' }));

		await waitFor(() => expect(registryQuery(stub.queries).get('q')).toBe('anthropic'));
		expect(registryQuery(stub.queries).get('page')).toBe('1');
		expect(await screen.findByText('Anthropic')).toBeTruthy();
		expect(screen.queryByText('OpenAI')).toBeNull();
	});

	it('sends a name the id does not carry, because the API matches both fields', async () => {
		const stub = stubProviders({
			rows: [
				provider(),
				provider({ id: 'remote-llm', name: 'Claude via Ollama', category: 'local' })
			]
		});
		render(ProvidersPage);
		await screen.findByRole('table');

		const input = screen.getByLabelText('Search providers by name or id');
		await fireEvent.input(input, { target: { value: 'ollama' } });
		await fireEvent.submit(input.closest('form') as HTMLFormElement);

		await waitFor(() => expect(registryQuery(stub.queries).get('q')).toBe('ollama'));
		expect(await screen.findByText('remote-llm')).toBeTruthy();
		expect(screen.queryByText('OpenAI')).toBeNull();
	});

	it('accepts the 120-code-point provider-search limit', async () => {
		const cases = [
			{ name: '120 ASCII code points', query: 'x'.repeat(120) },
			{ name: '120 astral Unicode code points', query: '😀'.repeat(120) },
			{ name: '120 mixed ASCII and Unicode code points', query: '😀'.repeat(60) + 'x'.repeat(60) }
		];
		for (const { query } of cases) {
			cleanup();
			vi.unstubAllGlobals();
			const stub = stubProviders({ rows: [provider()] });
			render(ProvidersPage);
			await screen.findByRole('table');

			const input = screen.getByLabelText('Search providers by name or id');
			await fireEvent.input(input, { target: { value: query } });
			await fireEvent.submit(input.closest('form') as HTMLFormElement);

			await waitFor(() => expect(registryQuery(stub.queries).get('q')).toBe(query));
			expect(screen.queryByRole('alert')).toBeNull();
		}
	});

	it('shows inline validation and sends no provider-search term over 120 code points', async () => {
		const cases = [
			{ name: '121 ASCII code points', query: 'x'.repeat(121) },
			{ name: '121 astral Unicode code points', query: '😀'.repeat(121) },
			{ name: '121 mixed ASCII and Unicode code points', query: '😀'.repeat(60) + 'x'.repeat(61) }
		];
		for (const { query } of cases) {
			cleanup();
			vi.unstubAllGlobals();
			const stub = stubProviders({ rows: [provider()] });
			render(ProvidersPage);
			await screen.findByRole('table');

			const initialReads = registryReads(stub.queries).length;
			const input = screen.getByLabelText('Search providers by name or id');
			await fireEvent.input(input, { target: { value: query } });
			await fireEvent.submit(input.closest('form') as HTMLFormElement);

			const validationMessage = await screen.findByRole('alert');
			expect(validationMessage.textContent).toContain('Search must be 120 characters or fewer.');
			expect(input.getAttribute('aria-invalid')).toBe('true');
			expect(registryReads(stub.queries)).toHaveLength(initialReads);
		}
	});

	it('drops the search parameter on an empty submit instead of sending it empty', async () => {
		const stub = stubProviders({ rows: [provider()] });
		render(ProvidersPage);
		await screen.findByRole('table');

		const input = screen.getByLabelText('Search providers by name or id');
		await fireEvent.input(input, { target: { value: '   ' } });
		await fireEvent.submit(input.closest('form') as HTMLFormElement);

		await waitFor(() => expect(registryQuery(stub.queries).has('q')).toBe(false));
	});

	it('sends the search and the category together, because the API filters with both', async () => {
		const stub = stubProviders({
			rows: [provider(), provider({ id: 'anthropic', name: 'Anthropic', category: 'media' })]
		});
		render(ProvidersPage);
		await screen.findByRole('table');

		await fireEvent.input(screen.getByLabelText('Search providers by name or id'), {
			target: { value: 'anthropic' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Search' }));
		await waitFor(() => expect(registryQuery(stub.queries).get('q')).toBe('anthropic'));

		await fireEvent.click(screen.getByRole('button', { name: 'media' }));
		await waitFor(() => expect(registryQuery(stub.queries).get('category')).toBe('media'));
		expect(registryQuery(stub.queries).get('q')).toBe('anthropic');
	});
});
