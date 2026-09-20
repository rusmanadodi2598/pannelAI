// Custom model write tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The page is the unit under test for the same reason as the disabled-set tests: a custom row joins the
// catalog, so the form's success has to reach the list above it. The stub refuses what the server refuses,
// which is what makes the local-refusal tests meaningful: a test that presses Add with a bad model id and
// asserts no request was sent proves the panel caught it, not that the stub was lenient.

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { catalogRow, customRow, stubModels, type ModelStub } from '../support/model-stub';
import { squashed, value } from '../support/dom';
import { forEachCase } from '../support/tables';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({
		catalog: [catalogRow()],
		custom: [
			customRow(),
			customRow({
				id: 'mdl_02',
				provider_id: 'anthropic',
				model_id: 'claude-3-haiku',
				display_name: 'Claude 3 Haiku'
			})
		]
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderProvider(providerId = 'openai'): void {
	render(ProviderDetailPage, { props: { params: { provider_id: providerId }, data: {} } });
}

/** Fills the add form. The fields are found by their label, so a renamed field fails here. */
function fillForm(fields: {
	model_id?: string;
	display_name?: string;
	capabilities?: string;
}): void {
	if (fields.model_id !== undefined) {
		const input = screen.getByLabelText('Model id') as HTMLInputElement;
		input.value = fields.model_id;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
	if (fields.display_name !== undefined) {
		const input = screen.getByLabelText('Display name') as HTMLInputElement;
		input.value = fields.display_name;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
	if (fields.capabilities !== undefined) {
		const input = screen.getByLabelText('Capabilities (optional)') as HTMLInputElement;
		input.value = fields.capabilities;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
}

async function waitForCustomTable(): Promise<void> {
	await waitFor(() =>
		expect(screen.getByRole('table', { name: /custom models declared/i })).toBeTruthy()
	);
}

describe('the custom model list', () => {
	it('lists this provider only, with a count that says so', async () => {
		renderProvider();
		await waitForCustomTable();

		const table = screen.getByRole('table', { name: /custom models declared/i });
		expect(within(table).getByText('GPT-4o mini')).toBeTruthy();
		expect(within(table).queryByText('Claude 3 Haiku')).toBeNull();
		expect(screen.getByText(/Another provider's rows are not listed here/)).toBeTruthy();
	});

	it('renders an empty state when this provider declares none', async () => {
		renderProvider('google');
		await waitFor(() =>
			expect(screen.getByText(/No custom models for this provider/)).toBeTruthy()
		);
	});

	it('states the capabilities a row declares', async () => {
		renderProvider();
		await waitForCustomTable();

		const table = screen.getByRole('table', { name: /custom models declared/i });
		expect(within(table).getByText('tools')).toBeTruthy();
	});
});

describe('adding a custom model', () => {
	it('sends the provider from the screen, the parsed capabilities, and shows the row', async () => {
		renderProvider();
		await waitForCustomTable();

		fillForm({
			model_id: 'gpt-5-mini',
			display_name: 'GPT-5 mini',
			capabilities: 'vision, tools, vision'
		});
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() => expect(stub.customCreates).toHaveLength(1));
		expect(stub.customCreates[0]).toEqual({
			provider_id: 'openai',
			model_id: 'gpt-5-mini',
			display_name: 'GPT-5 mini',
			capabilities: ['vision', 'tools']
		});

		await waitFor(() => expect(screen.getByText('GPT-5 mini')).toBeTruthy());
	});

	it('adds the new model to the catalog, because a custom row is a catalog row', async () => {
		renderProvider();
		await waitForCustomTable();

		fillForm({ model_id: 'gpt-5-mini', display_name: 'GPT-5 mini' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() => {
			const catalog = screen.getByRole('table', { name: /model catalog/i });
			expect(within(catalog).getByText('GPT-5 mini')).toBeTruthy();
		});
	});

	it('clears the form after a successful add', async () => {
		renderProvider();
		await waitForCustomTable();

		fillForm({ model_id: 'gpt-5-mini', display_name: 'GPT-5 mini', capabilities: 'vision' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() => expect(screen.getByText('GPT-5 mini')).toBeTruthy());
		expect(value(screen.getByLabelText('Model id'))).toBe('');
		expect(value(screen.getByLabelText('Display name'))).toBe('');
		expect(value(screen.getByLabelText('Capabilities (optional)'))).toBe('');
	});

	it('names the model it added, so the list change is not the only signal', async () => {
		renderProvider();
		await waitForCustomTable();

		fillForm({ model_id: 'gpt-5-mini', display_name: 'GPT-5 mini' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() =>
			expect(screen.getByText('openai/gpt-5-mini was added to the catalog.')).toBeTruthy()
		);
	});

	describe('refuses locally what the API would refuse', () => {
		forEachCase(
			[
				{
					name: 'a model id with a space',
					fields: { model_id: 'gpt 5', display_name: 'GPT-5' },
					message: 'A model id cannot contain spaces.'
				},
				{
					name: 'an empty model id',
					fields: { model_id: '', display_name: 'GPT-5' },
					message: 'A model id is required.'
				},
				{
					name: 'an empty display name',
					fields: { model_id: 'gpt-5', display_name: '' },
					message: 'A name is required.'
				},
				{
					name: 'a model id past the API bound',
					fields: { model_id: 'a'.repeat(201), display_name: 'GPT-5' },
					message: 'Use 200 characters or fewer.'
				}
			],
			async ({ fields, message }) => {
				renderProvider();
				await waitForCustomTable();

				fillForm(fields);
				screen.getByRole('button', { name: 'Add the model' }).click();

				const issues = await screen.findByRole('alert');
				expect(squashed(issues)).toContain(message);
				expect(stub.customCreates).toEqual([]);
			}
		);
	});

	it('reports the server refusal, including a pair that is already declared', async () => {
		renderProvider();
		await waitForCustomTable();

		// `openai/gpt-4o` is in the catalog, so the server answers 409 for the duplicate pair.
		fillForm({ model_id: 'gpt-4o', display_name: 'GPT-4o again' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() =>
			expect(screen.getByText(/model already exists: models_custom_pair/)).toBeTruthy()
		);
	});
});

describe('removing a custom model', () => {
	it('names the model and what happens to a shadowed registry row before it deletes', async () => {
		renderProvider();
		await waitForCustomTable();

		screen.getByRole('button', { name: 'Remove' }).click();

		const dialog = await screen.findByRole('dialog');
		expect(squashed(dialog)).toContain('Remove GPT-4o mini (openai/gpt-4o-mini)');
		expect(squashed(dialog)).toContain("the registry's version is what the catalog lists");
		expect(stub.customDeletes).toEqual([]);
	});

	it('deletes the row it named, and drops it from the list', async () => {
		renderProvider();
		await waitForCustomTable();

		screen.getByRole('button', { name: 'Remove' }).click();
		const dialog = await screen.findByRole('dialog');
		within(dialog).getByRole('button', { name: 'Remove the model' }).click();

		await waitFor(() => expect(stub.customDeletes).toEqual(['mdl_01']));
		await waitFor(() =>
			expect(screen.getByText(/No custom models for this provider/)).toBeTruthy()
		);
	});

	it('keeps the row and reports the failure inside the dialog when the delete fails', async () => {
		stub.writeStatus = 500;
		renderProvider();
		await waitForCustomTable();

		screen.getByRole('button', { name: 'Remove' }).click();
		const dialog = await screen.findByRole('dialog');
		within(dialog).getByRole('button', { name: 'Remove the model' }).click();

		await waitFor(() =>
			expect(squashed(screen.getByRole('dialog'))).toContain('The row could not be removed.')
		);
		// Scoped to the custom table: the same model is also a catalog row, so an unscoped query would
		// pass even if the row had gone from this table.
		const table = screen.getByRole('table', { name: /custom models declared/i });
		expect(within(table).getByText('GPT-4o mini')).toBeTruthy();
	});

	it('cancels without a request', async () => {
		renderProvider();
		await waitForCustomTable();

		screen.getByRole('button', { name: 'Remove' }).click();
		const dialog = await screen.findByRole('dialog');
		within(dialog).getByRole('button', { name: 'Keep it' }).click();

		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(stub.customDeletes).toEqual([]);
	});
});
