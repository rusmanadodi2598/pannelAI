// Repeatable row mode tests (docs/SPEC-UI/001-SPEC-UI.md §6.2, §8.1, §8.8.5).
//
// The stub answers the bulk route the way the gateway does, refusal included: a refused batch reports every
// row by index with the message on the offending one, and nothing is stored. That is what the cases below
// turn on, because a form that showed only the envelope would leave the operator to guess which row to fix,
// and a form that cleared its rows on a refusal would lose the batch it has to correct.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { forEachCase } from '../support/tables';
import BulkAddKeysForm from '../../src/lib/components/BulkAddKeysForm.svelte';
import { stubEndpoints } from '../support/endpoint-stub';
import { schemaBulkKeyRefusal } from '$lib/schemas/endpoint-bulk';

/** The submit control, which is the only button whose name starts with "Add " and a count. */
function submitButton(): HTMLButtonElement {
	return screen.getByRole('button', { name: /^Add \d+ keys?$/ }) as HTMLButtonElement;
}

function renderForm(onadded = vi.fn()): { onadded: () => void } {
	render(BulkAddKeysForm, { props: { endpointId: 'ep_1', onadded } });
	return { onadded };
}

function fill(index: number, label: string, value: string): Promise<unknown> {
	return Promise.all([
		fireEvent.input(screen.getByLabelText(`Label for row ${index}`), { target: { value: label } }),
		fireEvent.input(screen.getByLabelText(`Key for row ${index}`), { target: { value } })
	]);
}

async function addRow(): Promise<void> {
	await fireEvent.click(screen.getByRole('button', { name: 'Add another row' }));
}

async function submit(): Promise<void> {
	await fireEvent.click(submitButton());
}

/** The stub's answer for a batch it refuses whole, which is the case the row attribution exists for. */
function refusedStub(): void {
	stubEndpoints({
		bulkAnswer: {
			kind: 'refused',
			index: 1,
			code: 'VALIDATION_ERROR',
			message: 'duplicate label in batch'
		}
	});
}

describe('BulkAddKeysForm', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('sends every row in one request and reports the batch', async () => {
		const stub = stubEndpoints({ bulkAnswer: { kind: 'created', count: 2 } });
		const { onadded } = renderForm();

		await addRow();
		await fill(1, 'First', 'sk-aaaaaaaa');
		await fill(2, 'Second', 'sk-bbbbbbbb');
		await submit();

		expect(await screen.findByText('2 keys were added.')).toBeTruthy();
		expect(stub.bulkWrites).toHaveLength(1);
		expect(stub.bulkWrites[0].body).toEqual({
			keys: [
				{ label: 'First', value: 'sk-aaaaaaaa' },
				{ label: 'Second', value: 'sk-bbbbbbbb' }
			]
		});
		expect(onadded).toHaveBeenCalledTimes(1);
		// The rows are cleared, so a second press cannot resubmit the batch.
		expect(submitButton().textContent).toContain('Add 1 key');
	});

	it('keeps every row when the batch is refused whole, and says nothing was stored', async () => {
		refusedStub();
		const { onadded } = renderForm();

		await addRow();
		await fill(1, 'First', 'sk-aaaaaaaa');
		await fill(2, 'First', 'sk-bbbbbbbb');
		await submit();

		expect(await screen.findByText(/Nothing was added: duplicate label in batch/)).toBeTruthy();
		expect(onadded).not.toHaveBeenCalled();
		expect((screen.getByLabelText('Label for row 2') as HTMLInputElement).value).toBe('First');
		expect((screen.getByLabelText('Key for row 2') as HTMLInputElement).value).toBe('sk-bbbbbbbb');
	});

	it('links the refused row message to that row, not to the form', async () => {
		refusedStub();
		renderForm();

		await addRow();
		await fill(1, 'First', 'sk-aaaaaaaa');
		await fill(2, 'First', 'sk-bbbbbbbb');
		await submit();

		await screen.findByText(/Nothing was added/);

		// §8.8.5: the message is linked to its input, not only coloured.
		const describedBy = screen.getByLabelText('Key for row 2').getAttribute('aria-describedby');
		expect(describedBy).toBeTruthy();
		expect(document.getElementById(describedBy ?? '')?.textContent).toContain(
			'duplicate label in batch'
		);
		expect(screen.getByLabelText('Key for row 1').getAttribute('aria-describedby')).toBeNull();
	});

	forEachCase(
		[
			{ name: 'sends nothing when a row carries an empty key', value: '' },
			{ name: 'sends nothing when a row carries a key that is too short', value: 'short' }
		],
		async (testCase) => {
			const stub = stubEndpoints();
			const { onadded } = renderForm();

			await fill(1, 'First', testCase.value);
			await submit();

			expect(await screen.findByText('Nothing was sent: fix the rows marked below.')).toBeTruthy();
			expect(stub.bulkWrites).toHaveLength(0);
			expect(onadded).not.toHaveBeenCalled();
			expect(screen.getByLabelText('Key for row 1').getAttribute('aria-describedby')).toBeTruthy();
		}
	);

	it('adds and removes rows, and will not remove the last one', async () => {
		stubEndpoints();
		renderForm();

		expect(
			(screen.getByRole('button', { name: 'Remove row 1' }) as HTMLButtonElement).disabled
		).toBe(true);

		await addRow();
		expect(submitButton().textContent).toContain('Add 2 keys');

		await fireEvent.click(screen.getByRole('button', { name: 'Remove row 2' }));

		expect(screen.queryByLabelText('Key for row 2')).toBeNull();
		expect(submitButton().textContent).toContain('Add 1 key');
	});

	it('clears the outcome when the batch is edited, because it described the previous one', async () => {
		refusedStub();
		renderForm();

		await addRow();
		await fill(1, 'First', 'sk-aaaaaaaa');
		await fill(2, 'First', 'sk-bbbbbbbb');
		await submit();
		expect(await screen.findByText(/Nothing was added/)).toBeTruthy();

		await fireEvent.input(screen.getByLabelText('Key for row 2'), {
			target: { value: 'sk-cccccccc' }
		});

		await waitFor(() => {
			expect(screen.queryByText(/Nothing was added/)).toBeNull();
		});
		expect(screen.getByLabelText('Key for row 2').getAttribute('aria-describedby')).toBeNull();
	});
});

describe('schemaBulkKeyRefusal', () => {
	it('reads the served refusal with every row reported', () => {
		const parsed = schemaBulkKeyRefusal.safeParse({
			error: { code: 'VALIDATION_ERROR', message: 'duplicate label in batch' },
			results: [{ index: 0 }, { index: 1, error: 'duplicate label in batch' }]
		});

		expect(parsed.success).toBe(true);
	});

	it('refuses a refusal body with no row list, which is not the shape §8.1 defines', () => {
		const parsed = schemaBulkKeyRefusal.safeParse({
			error: { code: 'VALIDATION_ERROR', message: 'duplicate label in batch' }
		});

		expect(parsed.success).toBe(false);
	});
});
