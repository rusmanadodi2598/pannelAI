// Adding a custom model (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The stub refuses what the server refuses, which is what makes the local-refusal cases meaningful: a case
// that presses Add with a bad model id and asserts no request was sent proves the panel caught it, not that
// the stub was lenient. The list and the removal path are in `provider-custom-models.test.ts`.

import { cleanup, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	customModelStub,
	fillForm,
	renderProvider,
	waitForCustomTable
} from '../support/custom-model-page';
import { squashed, value } from '../support/dom';
import { forEachCase } from '../support/tables';
import type { ModelStub } from '../support/model-stub';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

let stub: ModelStub;

beforeEach(() => {
	stub = customModelStub();
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});
describe('adding a custom model', () => {
	it('sends the provider from the screen, the parsed capabilities, and shows the row', async () => {
		await renderProvider();
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
		await renderProvider();
		await waitForCustomTable();

		fillForm({ model_id: 'gpt-5-mini', display_name: 'GPT-5 mini' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() => {
			const catalog = screen.getByRole('table', { name: /model catalog/i });
			expect(within(catalog).getByText('GPT-5 mini')).toBeTruthy();
		});
	});

	it('clears the form after a successful add', async () => {
		await renderProvider();
		await waitForCustomTable();

		fillForm({ model_id: 'gpt-5-mini', display_name: 'GPT-5 mini', capabilities: 'vision' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() => expect(screen.getByText('GPT-5 mini')).toBeTruthy());
		expect(value(screen.getByLabelText('Model id'))).toBe('');
		expect(value(screen.getByLabelText('Display name (optional)'))).toBe('');
		expect(value(screen.getByLabelText('Capabilities (optional)'))).toBe('');
	});

	it('names the model it added, so the list change is not the only signal', async () => {
		await renderProvider();
		await waitForCustomTable();

		fillForm({ model_id: 'gpt-5-mini', display_name: 'GPT-5 mini' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() =>
			expect(screen.getByText('openai/gpt-5-mini was added to the catalog.')).toBeTruthy()
		);
	});

	// The wire requires a non-empty display name (`schema/model.go`, `CreateCustomModelRequest.DisplayName`)
	// while the reference adds a model from its id alone, so the body carries the id as the name. Blank is
	// also the state the form opens in, which is what makes this the case an operator hits first.
	it('stores the model id as the display name when the name is left blank', async () => {
		await renderProvider();
		await waitForCustomTable();

		fillForm({ model_id: 'gpt-5-mini' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() => expect(stub.customCreates).toHaveLength(1));
		expect(stub.customCreates[0]).toEqual({
			provider_id: 'openai',
			model_id: 'gpt-5-mini',
			display_name: 'gpt-5-mini',
			capabilities: []
		});
		// The screen names the row it added, which is what proves the write reached the page rather than
		// only the stub. The table's own cells are not queried: the id and the name are the same string
		// here, so a text query would match the row twice over.
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
					name: 'a model id past the API bound',
					fields: { model_id: 'a'.repeat(201), display_name: 'GPT-5' },
					message: 'Use 200 characters or fewer.'
				}
			],
			async ({ fields, message }) => {
				await renderProvider();
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
		await renderProvider();
		await waitForCustomTable();

		// `openai/gpt-4o` is in the catalog, so the server answers 409 for the duplicate pair.
		fillForm({ model_id: 'gpt-4o', display_name: 'GPT-4o again' });
		screen.getByRole('button', { name: 'Add the model' }).click();

		await waitFor(() =>
			expect(screen.getByText(/model already exists: models_custom_pair/)).toBeTruthy()
		);
	});
});
