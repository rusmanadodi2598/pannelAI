// Combo test tests (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The test is a modal whose click spends one account per reference, so two things are asserted against the
// real render rather than against a helper: nothing is sent until the operator confirms, and the readout
// reports each reference's own outcome instead of the combo's aggregate one.
//
// The last test is here because this slice made it real: the table's test dialog and the tab's delete
// dialog now coexist on `/combos`, and `Modal` used one fixed id for its title, so the two would have named
// each other by whichever came first in the document.

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import CombosTab from '../../src/lib/components/CombosTab.svelte';
import { comboProbeRow, comboRow, stubModels, type ModelStub } from '../support/model-stub';

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({
		combos: [
			comboRow({
				name: 'daily',
				models: [{ ref: 'openai/gpt-4o', priority: 0 }]
			})
		],
		comboTestResults: [comboProbeRow({ ref: 'smart' })]
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderTab(): void {
	render(CombosTab);
}

async function waitForRow(): Promise<void> {
	await waitFor(() =>
		expect(screen.getAllByRole('button', { name: 'Test' }).length).toBeGreaterThan(0)
	);
}

function rowTestButton(index = 0): HTMLElement {
	return screen.getAllByRole('button', { name: 'Test' })[index];
}

async function openTestDialog(index = 0): Promise<HTMLElement> {
	await waitForRow();
	rowTestButton(index).click();
	return screen.findByRole('dialog', { name: 'Test this combo' });
}

function confirmButton(): HTMLButtonElement {
	return screen.getByRole('button', {
		name: /Test the chain|Test again|Testing/
	}) as HTMLButtonElement;
}

describe('opening the test', () => {
	it('offers the action on every row and sends nothing until it is confirmed', async () => {
		stub.combos = [
			comboRow({ id: 'cmb_01', name: 'daily' }),
			comboRow({ id: 'cmb_02', name: 'nightly' })
		];
		renderTab();
		await waitFor(() => expect(screen.getAllByRole('button', { name: 'Test' }).length).toBe(2));

		const dialog = await openTestDialog();

		// The spend is stated before it happens: the probe writes no usage row, so the sentence is the only
		// warning the operator gets.
		expect(within(dialog).getByText(/spends one account at a time/)).toBeTruthy();
		expect(within(dialog).getByText(/not recorded in usage/)).toBeTruthy();
		expect(stub.comboTests).toEqual([]);
	});

	it('names the combo and its strategy in the question', async () => {
		renderTab();
		const dialog = await openTestDialog();

		expect(within(dialog).getByText('daily')).toBeTruthy();
		expect(within(dialog).getByText(/Fallback/)).toBeTruthy();
	});
});

describe('running the test', () => {
	it('sends one chain and reports what each reference resolved to', async () => {
		stub.comboTestResults = [
			comboProbeRow({ ref: 'smart', provider_id: 'openai', model_id: 'gpt-4o' })
		];
		renderTab();
		const dialog = await openTestDialog();

		confirmButton().click();

		await waitFor(() => expect(stub.comboTests).toEqual(['cmb_01']));
		// The ref as stored and the identity it resolved to are two facts, and an alias is exactly the case
		// where they differ.
		expect(await within(dialog).findByText('The only reference answered.')).toBeTruthy();
		expect(within(dialog).getByText('smart')).toBeTruthy();
		// The identity shares its line with the endpoint, so it is matched as a pattern rather than as the
		// element's whole text.
		expect(within(dialog).getByText(/openai\/gpt-4o/)).toBeTruthy();
		expect(within(dialog).getByText(/Answered in 120 ms/)).toBeTruthy();
		expect(within(dialog).getByText(/via ep_01/)).toBeTruthy();
	});

	it('reports a failed reference as a result rather than as a route error', async () => {
		stub.comboTestResults = [
			comboProbeRow({ ref: 'openai/gpt-4o' }),
			comboProbeRow({
				ref: 'anthropic/claude',
				ok: false,
				provider_id: '',
				model_id: '',
				endpoint_id: '',
				latency_ms: 0,
				error_code: 'UPSTREAM_ERROR',
				error: 'the member is down'
			})
		];
		renderTab();
		const dialog = await openTestDialog();

		confirmButton().click();

		expect(await within(dialog).findByText('1 of 2 references answered.')).toBeTruthy();
		expect(within(dialog).getByText(/Failed in 0 ms/)).toBeTruthy();
		expect(within(dialog).getByText(/UPSTREAM_ERROR: the member is down/)).toBeTruthy();
		// A probe that failed before it resolved has no identity, and the panel says so rather than leaving a
		// blank that reads as a defect.
		expect(within(dialog).getByText('No identity was reported.')).toBeTruthy();
	});

	it('labels a fusion combo judge so it is not read as a chain member', async () => {
		stub.combos = [comboRow({ strategy: 'fusion', judge_model: 'openai/gpt-4o' })];
		stub.comboTestResults = [
			comboProbeRow({ ref: 'openai/gpt-4o' }),
			comboProbeRow({ ref: 'openai/gpt-4o', role: 'judge', model_id: 'gpt-4o' })
		];
		renderTab();
		const dialog = await openTestDialog();

		confirmButton().click();

		expect(await within(dialog).findByText('Judge')).toBeTruthy();
		expect(within(dialog).getByText('Model')).toBeTruthy();
	});

	it('says a combo that stores no references had nothing to probe', async () => {
		stub.comboTestResults = [];
		renderTab();
		const dialog = await openTestDialog();

		confirmButton().click();

		expect(await within(dialog).findByText(/nothing to probe/)).toBeTruthy();
		expect(within(dialog).queryByRole('listitem')).toBeNull();
	});

	it('reports a route error without claiming the combo was deleted', async () => {
		stub.comboTestStatus = 404;
		renderTab();
		const dialog = await openTestDialog();

		confirmButton().click();

		expect(await within(dialog).findByText(/The combo could not be tested/)).toBeTruthy();
		expect(within(dialog).getByText(/combo not found/)).toBeTruthy();
		expect(within(dialog).queryByText(/answered/)).toBeNull();
	});
});

describe('the spend while a probe runs', () => {
	it('keeps one click to one chain and says it is testing', async () => {
		const stubFetch = globalThis.fetch;
		let release = (): void => {};
		vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
			const response = await stubFetch(input, init);
			if (String(input).endsWith('/test')) {
				await new Promise<void>((resolve) => {
					release = resolve;
				});
			}
			return response;
		});

		renderTab();
		const dialog = await openTestDialog();
		confirmButton().click();

		await waitFor(() => expect(stub.comboTests).toEqual(['cmb_01']));
		const running = within(dialog).getByRole('button', { name: 'Testing' }) as HTMLButtonElement;
		expect(running.disabled).toBe(true);
		running.click();
		expect(stub.comboTests).toEqual(['cmb_01']);

		release();
		expect(await within(dialog).findByText('The only reference answered.')).toBeTruthy();
		expect(within(dialog).getByRole('button', { name: 'Test again' })).toBeTruthy();
	});
});

describe('reopening the test', () => {
	it('starts clean, because the report belonged to the dialog that closed', async () => {
		renderTab();
		const dialog = await openTestDialog();
		confirmButton().click();
		await within(dialog).findByText('The only reference answered.');

		within(dialog).getByRole('button', { name: 'Close' }).click();
		await waitFor(() =>
			expect(screen.queryByRole('dialog', { name: 'Test this combo' })).toBeNull()
		);

		const reopened = await openTestDialog();
		expect(within(reopened).queryByText('The only reference answered.')).toBeNull();
		expect(within(reopened).getByRole('button', { name: 'Test the chain' })).toBeTruthy();
	});
});

describe('two dialogs on one screen', () => {
	it('names each dialog by its own title', async () => {
		renderTab();
		await waitForRow();

		screen.getByRole('button', { name: 'Delete' }).click();

		// The delete dialog comes second in the document, so a shared title id would have named it with the
		// test dialog's heading.
		const dialog = await screen.findByRole('dialog', { name: 'Delete this combo' });
		expect(within(dialog).getByRole('heading', { name: 'Delete this combo' })).toBeTruthy();
	});
});
