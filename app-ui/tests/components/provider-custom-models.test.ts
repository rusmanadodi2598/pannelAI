// Custom model reads and removal (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The page is the unit under test because a custom row joins the catalog, so the list has to be read where
// the operator reads it. The add path, with the refusals the panel proves before the wire, has its own file,
// `provider-custom-models-add.test.ts`.

import { cleanup, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { customModelStub, renderProvider, waitForCustomTable } from '../support/custom-model-page';
import { expectIconOnly } from '../support/icon-only';
import { squashed } from '../support/dom';
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
describe('the custom model list', () => {
	it('lists this provider only, with a count that says so', async () => {
		await renderProvider();
		await waitForCustomTable();

		const table = screen.getByRole('table', { name: /custom models declared/i });
		expect(within(table).getByText('GPT-4o mini')).toBeTruthy();
		expect(within(table).queryByText('Claude 3 Haiku')).toBeNull();
		expect(screen.getByText(/Other providers not listed here/)).toBeTruthy();
	});

	it('renders an empty state when this provider declares none', async () => {
		renderProvider('google');
		await waitFor(() =>
			expect(screen.getByText(/No custom models for this provider/)).toBeTruthy()
		);
	});

	it('states the capabilities a row declares', async () => {
		await renderProvider();
		await waitForCustomTable();

		const table = screen.getByRole('table', { name: /custom models declared/i });
		expect(within(table).getByText('tools')).toBeTruthy();
	});
});

describe('removing a custom model', () => {
	it('names the model and what happens to a shadowed registry row before it deletes', async () => {
		await renderProvider();
		await waitForCustomTable();

		expectIconOnly(screen.getByRole('button', { name: 'Remove' }), 'Remove');
		screen.getByRole('button', { name: 'Remove' }).click();

		const dialog = await screen.findByRole('dialog');
		expect(squashed(dialog)).toContain('Remove GPT-4o mini (openai/gpt-4o-mini)');
		expect(squashed(dialog)).toContain('If a registry model shares this name');
		expect(stub.customDeletes).toEqual([]);
	});

	it('deletes the row it named, and drops it from the list', async () => {
		await renderProvider();
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
		await renderProvider();
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
		await renderProvider();
		await waitForCustomTable();

		screen.getByRole('button', { name: 'Remove' }).click();
		const dialog = await screen.findByRole('dialog');
		within(dialog).getByRole('button', { name: 'Keep it' }).click();

		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(stub.customDeletes).toEqual([]);
	});
});
