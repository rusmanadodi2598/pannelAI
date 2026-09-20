// Model catalog component tests (docs/SPEC-UI/001-SPEC-UI.md §6.3, §8.3).
//
// The catalog has two empty states and they are not the same problem: an operator whose filters matched
// nothing needs to clear a filter, and an operator whose provider offers no models needs to look at the
// registry entry. §8.3 requires the cause to be named, so both paths are asserted here rather than left to
// the template, along with the query the component actually sends, because a filter the panel silently
// drops would look identical to a filter that matched nothing.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ModelCatalogList from '../../src/lib/components/ModelCatalogList.svelte';
import { createModelDisabledStore } from '../../src/lib/stores/model-disabled.svelte';

function catalogRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		kind: 'llm',
		capabilities: ['vision', 'tools'],
		source: 'registry',
		...overrides
	};
}

/**
 * Replaces `fetch` with a stub that records every URL it was asked for, so a test can assert both what
 * the panel rendered and what it requested. The stub is the whole boundary: nothing below it is mocked.
 */
function stubCatalog(data: Record<string, unknown>[], status = 200): string[] {
	const requested: string[] = [];

	vi.stubGlobal('fetch', async (input: unknown) => {
		requested.push(String(input));
		return new Response(JSON.stringify({ data }), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return requested;
}

/** Renders the component and waits until the first request has been answered. */
async function renderCatalog(
	data: Record<string, unknown>[],
	status = 200
): Promise<{ requested: string[]; container: HTMLElement }> {
	const requested = stubCatalog(data, status);
	// The disabled set is the page's to load, so a store that was never loaded is what this component
	// sees here: it renders the Disable action and refuses the write, which the page-level tests cover.
	const { container } = render(ModelCatalogList, {
		props: {
			providerId: 'openai',
			disabled: createModelDisabledStore(),
			onchanged: () => {}
		}
	});

	await waitFor(() => {
		if (requested.length === 0) throw new Error('the catalog was never requested');
	});

	return { requested, container };
}

/** Submits the search form the way a keyboard user would. */
async function search(container: HTMLElement, term: string): Promise<void> {
	await fireEvent.input(within(container).getByLabelText('Search models'), {
		target: { value: term }
	});
	await fireEvent.submit(container.querySelector('form') as HTMLFormElement);
}

describe('ModelCatalogList', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('scopes the request to the provider it was handed', async () => {
		const { requested } = await renderCatalog([catalogRow()]);

		expect(requested[0]).toContain('provider_id=openai');
	});

	it('renders a row with its identity, kind, capabilities, and origin', async () => {
		await renderCatalog([catalogRow()]);

		expect(await screen.findByText('GPT-4o')).toBeTruthy();

		// Scoped to the table: `vision` and `tools` are also the labels of the two filter buttons, and a
		// screen-wide query would match the controls instead of the row.
		const table = within(screen.getByRole('table'));

		expect(table.getByText('gpt-4o')).toBeTruthy();
		expect(table.getByText('llm')).toBeTruthy();
		expect(table.getByText('vision')).toBeTruthy();
		expect(table.getByText('tools')).toBeTruthy();
		expect(table.getByText('Registry')).toBeTruthy();
	});

	it('leads with the model id when the row carries no display name', async () => {
		await renderCatalog([catalogRow({ display_name: '', model_id: 'gpt-4o-mini' })]);

		// The id appears once as the row's heading and once beneath it, so the label fallback and the raw id
		// are both present rather than one blank cell.
		expect(await screen.findAllByText('gpt-4o-mini')).toHaveLength(2);
	});

	it('says the provider offers no models when nothing came back and no filter is set', async () => {
		await renderCatalog([]);

		expect(await screen.findByText('This provider offers no models')).toBeTruthy();
		expect(screen.queryByText('No models found for this filter')).toBeNull();
	});

	it('names the filter, not the provider, once a filter is applied', async () => {
		const { container } = await renderCatalog([]);
		await screen.findByText('This provider offers no models');

		await search(container, 'does-not-exist');

		expect(await screen.findByText('No models found for this filter')).toBeTruthy();
		expect(screen.queryByText('This provider offers no models')).toBeNull();
	});

	it('sends a trimmed search term rather than what was typed', async () => {
		const { requested, container } = await renderCatalog([catalogRow()]);
		await screen.findByText('GPT-4o');

		await search(container, '  gpt 4o  ');

		await waitFor(() => {
			expect(requested.length).toBeGreaterThan(1);
		});

		expect(requested[requested.length - 1]).toContain('q=gpt+4o');
	});

	it('sends the capability parameter when a filter is pressed, and drops it when pressed again', async () => {
		const { requested } = await renderCatalog([catalogRow()]);
		const vision = screen.getByRole('button', { name: 'vision' });

		expect(vision.getAttribute('aria-pressed')).toBe('false');

		await fireEvent.click(vision);

		await waitFor(() => {
			expect(requested.length).toBeGreaterThan(1);
		});
		expect(requested[requested.length - 1]).toContain('capability=vision');
		expect(vision.getAttribute('aria-pressed')).toBe('true');

		await fireEvent.click(vision);

		await waitFor(() => {
			expect(requested.length).toBeGreaterThan(2);
		});
		expect(requested[requested.length - 1]).not.toContain('capability=');
		expect(vision.getAttribute('aria-pressed')).toBe('false');
	});

	it('offers a way out of the filter it applied', async () => {
		await renderCatalog([catalogRow()]);
		await screen.findByText('GPT-4o');

		// No filter is set, so there is nothing to clear and no control that would do nothing (R-26).
		expect(screen.queryByRole('button', { name: 'Clear filters' })).toBeNull();

		await fireEvent.click(screen.getByRole('button', { name: 'tools' }));

		expect(await screen.findByRole('button', { name: 'Clear filters' })).toBeTruthy();
	});

	it('reports a failure and offers a retry rather than an empty catalog', async () => {
		const requested = stubCatalog([], 500);
		render(ModelCatalogList, {
			props: {
				providerId: 'openai',
				disabled: createModelDisabledStore(),
				onchanged: () => {}
			}
		});

		expect(await screen.findByText('The model catalog could not be loaded')).toBeTruthy();
		expect(screen.queryByText('This provider offers no models')).toBeNull();

		const before = requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(requested.length).toBeGreaterThan(before);
		});
	});
});
