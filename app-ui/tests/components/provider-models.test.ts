// Provider detail model write tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The page is the unit under test, not the components, because the two halves of this feature share one
// store: disabling a model in the catalog must make it appear in the disabled list and take it out of the
// catalog, and only the page wires that up. A component test could pass while the page passed the wrong
// store to one of them.
//
// The stub applies every write to its own state and hides a disabled model from the catalog the way the
// server does, so a test that presses Disable and then looks for the row is testing the real rule.
//
// The outcome line is found by its text rather than by its role: StateMessage renders its loading and
// error states as live regions too, and a role query would match whichever of them happens to be on
// screen. The role is asserted separately, on the element the text belongs to.

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { catalogRow, stubModels, type ModelStub } from '../support/model-stub';
import { expectIconOnly } from '../support/icon-only';
import { squashed } from '../support/dom';
import { forEachCase } from '../support/tables';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

const ref = (provider_id: string, model_id: string): { provider_id: string; model_id: string } => ({
	provider_id,
	model_id
});

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({
		catalog: [
			catalogRow(),
			catalogRow({
				id: 'openai/gpt-4o-mini',
				model_id: 'gpt-4o-mini',
				display_name: 'GPT-4o mini'
			}),
			catalogRow({
				id: 'anthropic/claude-3-haiku',
				provider_id: 'anthropic',
				model_id: 'claude-3-haiku',
				display_name: 'Claude 3 Haiku'
			})
		],
		disabled: [ref('anthropic', 'claude-3-haiku')]
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderProvider(providerId = 'openai'): void {
	render(ProviderDetailPage, { props: { params: { provider_id: providerId }, data: {} } });
}

/** The row a model id belongs to, found through its own cell rather than by table position. */
function rowFor(modelId: string): HTMLElement {
	const cell = screen.getByText(modelId);
	const row = cell.closest('tr');
	expect(row, `no row carries ${modelId}`).toBeTruthy();
	return row as HTMLElement;
}

async function waitForCatalog(): Promise<void> {
	await waitFor(() => expect(screen.getByRole('table', { name: /model catalog/i })).toBeTruthy());
}

/** Waits for a sentence and returns the live region it sits in, whichever role that region uses. */
async function outcomeRegion(text: string | RegExp): Promise<HTMLElement> {
	const line = await screen.findByText(text);
	const region = line.closest('[role="status"], [role="alert"]');
	expect(region, `no live region carries ${text}`).toBeTruthy();
	return region as HTMLElement;
}

describe('the catalog', () => {
	it('offers a Disable action on every row it lists, rendered icon-only', async () => {
		renderProvider();
		await waitForCatalog();

		// Icon-only since 2026-09-25 (PORT 002 D7): the contract is shared with the key tables.
		expectIconOnly(within(rowFor('gpt-4o')).getByRole('button', { name: 'Disable' }), 'Disable');
		expect(within(rowFor('gpt-4o-mini')).getByRole('button', { name: 'Disable' })).toBeTruthy();
	});

	it('lists only this provider, so another provider has no row to disable', async () => {
		renderProvider();
		await waitForCatalog();

		expect(screen.queryByText('claude-3-haiku')).toBeNull();
	});

	it('disables a model by sending the whole set, another provider included', async () => {
		renderProvider();
		await waitForCatalog();

		within(rowFor('gpt-4o')).getByRole('button', { name: 'Disable' }).click();

		await waitFor(() => expect(stub.disabledWrites).toHaveLength(1));
		expect(stub.disabledWrites[0]).toEqual([
			ref('anthropic', 'claude-3-haiku'),
			ref('openai', 'gpt-4o')
		]);
	});

	it('takes the disabled model out of the catalog and shows it in the disabled list', async () => {
		renderProvider();
		await waitForCatalog();

		within(rowFor('gpt-4o')).getByRole('button', { name: 'Disable' }).click();

		await waitFor(() => {
			const disabled = screen.getByRole('table', { name: /disabled for this provider/i });
			expect(within(disabled).getByText('gpt-4o')).toBeTruthy();
		});
		await waitFor(() => expect(screen.queryByText('GPT-4o')).toBeNull());
	});

	it('reports the refusal beside the row that could not be disabled', async () => {
		stub.writeStatus = 500;
		renderProvider();
		await waitForCatalog();

		within(rowFor('gpt-4o')).getByRole('button', { name: 'Disable' }).click();

		const region = await outcomeRegion(/The set could not be stored/);
		expect(region.getAttribute('role')).toBe('alert');
		// The row is still here, which is where the message belongs.
		expect(screen.getByText('GPT-4o')).toBeTruthy();
	});

	it('refuses to write when the disabled set never loaded, and says why', async () => {
		stub.disabledReadStatus = 500;
		renderProvider();
		await waitForCatalog();

		within(rowFor('gpt-4o')).getByRole('button', { name: 'Disable' }).click();

		await outcomeRegion(/has not loaded/);
		expect(stub.disabledWrites).toEqual([]);
	});
});

describe('the disabled list', () => {
	it('lists this provider only, with a count that says so', async () => {
		renderProvider('anthropic');
		await waitFor(() =>
			expect(screen.getByRole('table', { name: /disabled for this provider/i })).toBeTruthy()
		);

		expect(screen.getByText('claude-3-haiku')).toBeTruthy();
		expect(screen.getByText(/Another provider's disabled models are not listed here/)).toBeTruthy();
	});

	it('turns a model back on and returns it to the catalog', async () => {
		renderProvider('anthropic');
		await waitFor(() =>
			expect(screen.getByRole('table', { name: /disabled for this provider/i })).toBeTruthy()
		);

		expectIconOnly(screen.getByRole('button', { name: 'Enable' }), 'Enable');
		screen.getByRole('button', { name: 'Enable' }).click();

		await waitFor(() => expect(stub.disabledWrites).toEqual([[]]));
		await waitFor(() => {
			const catalog = screen.getByRole('table', { name: /model catalog/i });
			expect(within(catalog).getByText('Claude 3 Haiku')).toBeTruthy();
		});
	});

	it('renders an empty state when nothing is disabled for this provider', async () => {
		renderProvider();
		await waitFor(() =>
			expect(screen.getByText(/No models are disabled for this provider/)).toBeTruthy()
		);
	});

	it('reports a failed read with a retry that re-reads', async () => {
		stub.disabledReadStatus = 500;
		renderProvider();
		await waitFor(() =>
			expect(screen.getByText(/The disabled models could not be loaded/)).toBeTruthy()
		);

		stub.disabledReadStatus = 200;
		screen.getByRole('button', { name: 'Try again' }).click();

		await waitFor(() =>
			expect(screen.getByText(/No models are disabled for this provider/)).toBeTruthy()
		);
	});
});

describe('the outcome line', () => {
	forEachCase(
		[
			{
				name: 'names the model that was just disabled, and it lands in the disabled list',
				provider: 'openai',
				model: 'gpt-4o',
				action: 'Disable',
				expected: 'openai/gpt-4o is disabled.'
			},
			{
				name: 'names the model that was turned back on, and it lands in the catalog',
				provider: 'anthropic',
				model: 'claude-3-haiku',
				action: 'Enable',
				expected: 'anthropic/claude-3-haiku is routable again.'
			}
		],
		async ({ provider, model, action, expected }) => {
			renderProvider(provider);
			await waitFor(() => expect(screen.getByText(model)).toBeTruthy());

			within(rowFor(model)).getByRole('button', { name: action }).click();

			const region = await outcomeRegion(expected);
			expect(region.getAttribute('role')).toBe('status');
			expect(squashed(region)).toBe(expected);
		}
	);
});
