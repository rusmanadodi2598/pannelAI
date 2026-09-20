// Alias write tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The page is the unit under test, for the same reason as the other provider detail sections: the alias
// table sits among four others, and a query that is not scoped to it would pass on another section's row.
//
// The set is global and the write replaces it whole, so the tests that matter are about what a write
// carries: adding one alias must send every alias that was already there, and removing one must send the
// rest. The stub applies the same refusals as the server, with the server's own sentences.

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { aliasRow, catalogRow, comboRow, stubModels, type ModelStub } from '../support/model-stub';
import { squashed, value } from '../support/dom';
import { forEachCase } from '../support/tables';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

let stub: ModelStub;

// The catalog the stub resolves alias targets against: the server refuses a target that is neither a known
// model nor a combo name, so a test target has to exist here for the write to land.
beforeEach(() => {
	stub = stubModels({
		catalog: [
			catalogRow(),
			catalogRow({ id: 'openai/gpt-4o-mini', provider_id: 'openai', model_id: 'gpt-4o-mini' }),
			catalogRow({
				id: 'anthropic/claude-3-haiku',
				provider_id: 'anthropic',
				model_id: 'claude-3-haiku'
			})
		],
		combos: [comboRow()],
		aliases: [
			aliasRow({ alias: 'fast', target: 'openai/gpt-4o' }),
			aliasRow({ alias: 'smart', target: 'fallback-combo' })
		]
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderProvider(): void {
	render(ProviderDetailPage, { props: { params: { provider_id: 'openai' }, data: {} } });
}

async function waitForAliasTable(): Promise<void> {
	await waitFor(() => expect(screen.getByRole('table', { name: /alias set/i })).toBeTruthy());
}

function aliasTable(): HTMLElement {
	return screen.getByRole('table', { name: /alias set/i });
}

/** Fills the add form. The fields are found by their label, so a renamed field fails here. */
function fillForm(fields: { alias?: string; target?: string }): void {
	if (fields.alias !== undefined) {
		const input = screen.getByLabelText('Alias') as HTMLInputElement;
		input.value = fields.alias;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
	if (fields.target !== undefined) {
		const input = screen.getByLabelText('Target') as HTMLInputElement;
		input.value = fields.target;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
}

describe('the alias table', () => {
	it('lists every alias with its target and says the set is global', async () => {
		renderProvider();
		await waitForAliasTable();

		expect(within(aliasTable()).getByText('fast')).toBeTruthy();
		expect(within(aliasTable()).getByText('openai/gpt-4o')).toBeTruthy();
		expect(within(aliasTable()).getByText('smart')).toBeTruthy();
		expect(within(aliasTable()).getByText('fallback-combo')).toBeTruthy();
		expect(screen.getByText(/every provider's detail screen shows this same table/)).toBeTruthy();
	});

	it('renders an empty state when the gateway holds no alias', async () => {
		stub.aliases = [];
		renderProvider();

		await waitFor(() => expect(screen.getByText(/No aliases yet/)).toBeTruthy());
		// The form stays: an empty set is the state an operator adds the first alias from.
		expect(screen.getByRole('button', { name: 'Add the alias' })).toBeTruthy();
	});

	it('reports a failed read and re-reads on request', async () => {
		stub.aliasReadStatus = 500;
		renderProvider();

		await waitFor(() => expect(screen.getByText(/The alias set could not be loaded/)).toBeTruthy());
		// The form waits for the set: a write the store would refuse is not offered.
		expect(screen.queryByRole('button', { name: 'Add the alias' })).toBeNull();

		stub.aliasReadStatus = 200;
		within(screen.getByRole('alert')).getByRole('button', { name: 'Try again' }).click();

		await waitForAliasTable();
	});
});

describe('adding an alias', () => {
	it('sends the whole set, the aliases that were already there included', async () => {
		renderProvider();
		await waitForAliasTable();

		fillForm({ alias: 'cheap', target: 'anthropic/claude-3-haiku' });
		screen.getByRole('button', { name: 'Add the alias' }).click();

		await waitFor(() => expect(stub.aliasWrites).toHaveLength(1));
		expect(stub.aliasWrites[0]).toEqual([
			{ alias: 'cheap', target: 'anthropic/claude-3-haiku' },
			{ alias: 'fast', target: 'openai/gpt-4o' },
			{ alias: 'smart', target: 'fallback-combo' }
		]);

		await waitFor(() => expect(within(aliasTable()).getByText('cheap')).toBeTruthy());
	});

	it('clears the form and names the alias it added', async () => {
		renderProvider();
		await waitForAliasTable();

		fillForm({ alias: 'cheap', target: 'openai/gpt-4o' });
		screen.getByRole('button', { name: 'Add the alias' }).click();

		await waitFor(() => expect(screen.getByText('cheap was added.')).toBeTruthy());
		expect(value(screen.getByLabelText('Alias'))).toBe('');
		expect(value(screen.getByLabelText('Target'))).toBe('');
	});

	it('changes what an existing alias targets rather than adding a second row', async () => {
		renderProvider();
		await waitForAliasTable();

		fillForm({ alias: 'fast', target: 'openai/gpt-4o-mini' });
		screen.getByRole('button', { name: 'Add the alias' }).click();

		await waitFor(() =>
			expect(screen.getByText('fast now targets openai/gpt-4o-mini.')).toBeTruthy()
		);
		expect(stub.aliasWrites[0]).toHaveLength(2);
		expect(within(aliasTable()).getByText('openai/gpt-4o-mini')).toBeTruthy();
		expect(within(aliasTable()).queryByText('openai/gpt-4o')).toBeNull();
	});

	it('reports the server refusal when the target resolves to nothing', async () => {
		renderProvider();
		await waitForAliasTable();

		fillForm({ alias: 'ghost', target: 'openai/ghost' });
		screen.getByRole('button', { name: 'Add the alias' }).click();

		await waitFor(() =>
			expect(screen.getByText(/targets an unknown model or combo: openai\/ghost/)).toBeTruthy()
		);
		// The set is unchanged: the API validated the whole body and refused it before writing.
		expect(within(aliasTable()).queryByText('ghost')).toBeNull();
	});

	it('reports the refusal when the alias is already a combo name', async () => {
		renderProvider();
		await waitForAliasTable();

		fillForm({ alias: 'fallback-combo', target: 'openai/gpt-4o' });
		screen.getByRole('button', { name: 'Add the alias' }).click();

		await waitFor(() =>
			expect(screen.getByText(/alias fallback-combo is already a combo name/)).toBeTruthy()
		);
	});

	describe('refuses locally what the API would refuse', () => {
		forEachCase(
			[
				{
					name: 'an empty alias',
					fields: { alias: '', target: 'openai/gpt-4o' },
					message: 'An alias is required.'
				},
				{
					name: 'an alias with a slash',
					fields: { alias: 'openai/fast', target: 'openai/gpt-4o' },
					message: 'An alias cannot contain a slash'
				},
				{
					name: 'an empty target',
					fields: { alias: 'cheap', target: '' },
					message: 'A target is required.'
				},
				{
					name: 'a target with a space',
					fields: { alias: 'cheap', target: 'openai/gpt 4o' },
					message: 'A target cannot contain spaces.'
				},
				{
					name: 'a target past the API bound',
					fields: { alias: 'cheap', target: `a/${'b'.repeat(199)}` },
					message: 'Use 200 characters or fewer.'
				}
			],
			async ({ fields, message }) => {
				renderProvider();
				await waitForAliasTable();

				fillForm(fields);
				screen.getByRole('button', { name: 'Add the alias' }).click();

				const issues = await screen.findByRole('alert');
				expect(squashed(issues)).toContain(message);
				expect(stub.aliasWrites).toEqual([]);
			}
		);
	});
});

describe('removing an alias', () => {
	it('sends the rest of the set and says what stopped resolving', async () => {
		renderProvider();
		await waitForAliasTable();

		const row = within(aliasTable()).getByText('smart').closest('tr') as HTMLElement;
		within(row).getByRole('button', { name: 'Remove' }).click();

		await waitFor(() =>
			expect(stub.aliasWrites).toEqual([[{ alias: 'fast', target: 'openai/gpt-4o' }]])
		);
		await waitFor(() => expect(screen.getByText('smart no longer resolves.')).toBeTruthy());
		expect(within(aliasTable()).queryByText('smart')).toBeNull();
	});

	it('sends an empty set when the last alias goes', async () => {
		stub.aliases = [aliasRow({ alias: 'fast', target: 'openai/gpt-4o' })];
		renderProvider();
		await waitForAliasTable();

		within(aliasTable()).getByRole('button', { name: 'Remove' }).click();

		await waitFor(() => expect(stub.aliasWrites).toEqual([[]]));
		await waitFor(() => expect(screen.getByText(/No aliases yet/)).toBeTruthy());
	});

	it('keeps the row and reports the failure when the write fails', async () => {
		stub.writeStatus = 500;
		renderProvider();
		await waitForAliasTable();

		const row = within(aliasTable()).getByText('smart').closest('tr') as HTMLElement;
		within(row).getByRole('button', { name: 'Remove' }).click();

		await waitFor(() => expect(screen.getByText('The set could not be stored.')).toBeTruthy());
		expect(within(aliasTable()).getByText('smart')).toBeTruthy();
	});
});

describe('the target suggestions', () => {
	it('offers the catalog ids and the combo names', async () => {
		renderProvider();
		await waitForAliasTable();

		await waitFor(() => {
			const options = [...document.querySelectorAll('#alias-targets option')].map((option) =>
				option.getAttribute('value')
			);
			expect(options).toEqual([
				'anthropic/claude-3-haiku',
				'fallback-combo',
				'openai/gpt-4o',
				'openai/gpt-4o-mini'
			]);
		});
	});

	it('says the suggestions could not be read, and still takes a typed target', async () => {
		stub.readStatus = 500;
		renderProvider();
		await waitForAliasTable();

		await waitFor(() => expect(screen.getByText(/The suggestions could not be read/)).toBeTruthy());

		fillForm({ alias: 'cheap', target: 'openai/gpt-4o' });
		screen.getByRole('button', { name: 'Add the alias' }).click();

		await waitFor(() => expect(stub.aliasWrites).toHaveLength(1));
	});

	it('says when the combo names are truncated, rather than looking like the whole list', async () => {
		stub.combos = Array.from({ length: 120 }, (_unused, index) =>
			comboRow({ id: `cmb_${index}`, name: `combo-${index}` })
		);
		renderProvider();
		await waitForAliasTable();

		await waitFor(() =>
			expect(screen.getByText(/The combo names listed are the first 100 of 120/)).toBeTruthy()
		);
	});
});
