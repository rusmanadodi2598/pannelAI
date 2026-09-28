// Per-model test controls on the provider catalog (docs/SPEC-UI/001-SPEC-UI.md §6.3, draft 017 §4.10).
//
// The probe answers for one model, so the contract these tests hold is that an answer stays on the row it
// names: three rows, three verdicts, and a sweep that fills exactly the rows it probed and says how many it
// did not. A refusal is rendered as the server's own sentence rather than a paraphrase, because the panel
// has no better claim on why a model failed than the route that asked it (R-36).

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ModelCatalogList from '../../src/lib/components/ModelCatalogList.svelte';
import { createModelDisabledStore } from '../../src/lib/stores/model-disabled.svelte';
import { createProviderThinkingStore } from '../../src/lib/stores/provider-thinking.svelte';
import { catalogRow, stubModels, type ModelStub } from '../support/model-stub';
import { expectIconOnly } from '../support/icon-only';
import { squashed } from '../support/dom';
import { MODEL_TEST_SWEEP_LIMIT } from '$lib/schemas/model-test';

const chatRow = catalogRow();
const imageRow = catalogRow({
	id: 'openai/dall-e-3',
	model_id: 'dall-e-3',
	display_name: 'DALL-E 3',
	kind: 'image'
});

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({ catalog: [chatRow, imageRow] });
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

let view: ReturnType<typeof render>;

/** Renders the catalog and waits until its rows are on screen, not merely requested. */
async function renderCatalog(token = 0): Promise<HTMLElement> {
	view = render(ModelCatalogList, {
		props: {
			providerId: 'openai',
			disabled: createModelDisabledStore(),
			thinking: createProviderThinkingStore(),
			onchanged: () => {},
			token
		}
	});
	const container = view.container as HTMLElement;
	await waitFor(() => {
		expect(container.querySelector('tbody tr')).toBeTruthy();
	});
	return container;
}

function rowOf(container: HTMLElement, modelId: string): HTMLElement {
	const rows = [...container.querySelectorAll('tbody tr')];
	const match = rows.find((row) => row.textContent?.includes(modelId));
	if (!match) throw new Error(`no catalog row for ${modelId}`);
	return match as HTMLElement;
}

function testButton(row: HTMLElement): HTMLButtonElement {
	return within(row).getByRole('button', { name: /^Test/ }) as HTMLButtonElement;
}

describe('the catalog row test action', () => {
	it('offers the button only where a chat probe can reach the model', async () => {
		const container = await renderCatalog();

		// The copy control in the Model column also carries a `role="status"` for its confirmation, so the
		// probe's state is read by its own words rather than by the role.
		const chat = rowOf(container, 'gpt-4o');
		expect(squashed(within(chat).getByText('Not tested'))).toBe('Not tested');
		expect(testButton(chat)).toBeTruthy();

		const image = rowOf(container, 'dall-e-3');
		expect(squashed(within(image).getByText('Not a chat model'))).toBe('Not a chat model');
		expect(within(image).queryByRole('button', { name: /^Test/ })).toBeNull();
	});

	it('keeps the glyph decorative and the name on the button', async () => {
		const container = await renderCatalog();
		expectIconOnly(testButton(rowOf(container, 'gpt-4o')), 'Test');
	});

	it('asks for the model the row names and renders its answer', async () => {
		stub.modelTestRows = [
			{ model_id: 'gpt-4o', name: 'GPT-4o', ok: true, latency_ms: 214, endpoint_id: 'ep_01' }
		];
		const container = await renderCatalog();
		const row = rowOf(container, 'gpt-4o');

		await fireEvent.click(testButton(row));

		expect(stub.modelTests).toEqual(['openai/gpt-4o']);
		await waitFor(() => {
			expect(squashed(within(row).getByText('Answered in 214 ms'))).toBe('Answered in 214 ms');
		});
		expect(within(row).queryByText(/RATE_LIMITED/)).toBeNull();
	});

	it('reports a refusal with the code the gateway named', async () => {
		stub.modelTestRows = [
			{
				model_id: 'gpt-4o',
				ok: false,
				latency_ms: 8,
				status: 429,
				error_code: 'RATE_LIMITED',
				error: 'quota spent'
			}
		];
		const container = await renderCatalog();
		const row = rowOf(container, 'gpt-4o');

		await fireEvent.click(testButton(row));

		await waitFor(() => {
			const alert = within(row).getByRole('alert');
			expect(squashed(alert)).toContain('Failed in 8 ms');
			expect(squashed(alert)).toContain('RATE_LIMITED: quota spent');
		});
	});

	it('holds the row while the probe is in flight', async () => {
		const container = await renderCatalog();
		const row = rowOf(container, 'gpt-4o');
		const button = testButton(row);

		void fireEvent.click(button);

		expect(button.disabled).toBe(true);
		expect(squashed(within(row).getByText('Testing'))).toBe('Testing');

		await waitFor(() => {
			expect(button.disabled).toBe(false);
		});
	});

	it('says when the panel never got an answer, which is not the model failing', async () => {
		stub.modelTestStatus = 400;
		const container = await renderCatalog();
		const row = rowOf(container, 'gpt-4o');

		await fireEvent.click(testButton(row));

		await waitFor(() => {
			const alert = within(row).getByRole('alert');
			expect(squashed(alert)).toContain('The panel could not ask');
			expect(squashed(alert)).toContain('The model test was refused.');
		});
	});
});

describe('the bounded sweep', () => {
	function sweepButton(): HTMLButtonElement {
		return screen.getByRole('button', {
			name: new RegExp(`Test the first ${MODEL_TEST_SWEEP_LIMIT} models`)
		}) as HTMLButtonElement;
	}

	it('sends the budget the button states', async () => {
		stub.modelTestRows = [
			{ model_id: 'gpt-4o', ok: true, latency_ms: 30 },
			{ model_id: 'gpt-4o-mini', ok: false, latency_ms: 5, error_code: 'RATE_LIMITED' }
		];
		stub.modelTestTotal = 9;
		await renderCatalog();

		await fireEvent.click(sweepButton());

		expect(stub.modelSweeps).toEqual([{ provider: 'openai', limit: MODEL_TEST_SWEEP_LIMIT }]);
		await waitFor(() => {
			expect(squashed(screen.getByText(/Tested 2 of 9 models/))).toBe(
				'Tested 2 of 9 models: 1 answered, 1 failed.'
			);
		});
	});

	it('fills the rows it probed and leaves the others untested', async () => {
		stub.modelTestRows = [{ model_id: 'gpt-4o', name: 'GPT-4o', ok: true, latency_ms: 30 }];
		stub.modelTestTotal = 9;
		const container = await renderCatalog();

		await fireEvent.click(sweepButton());

		await waitFor(() => {
			expect(squashed(within(rowOf(container, 'gpt-4o')).getByText('Answered in 30 ms'))).toBe(
				'Answered in 30 ms'
			);
		});
		expect(squashed(within(rowOf(container, 'dall-e-3')).getByText('Not a chat model'))).toBe(
			'Not a chat model'
		);
	});

	it('names how much of the catalog the sweep covered', async () => {
		stub.modelTestRows = [{ model_id: 'gpt-4o', ok: true, latency_ms: 30 }];
		stub.modelTestTotal = 9;
		await renderCatalog();

		await fireEvent.click(sweepButton());

		await waitFor(() => {
			expect(squashed(screen.getByText(/Tested 1 of 9 models/))).toBe(
				'Tested 1 of 9 models: all answered.'
			);
		});
	});

	it('says when the budget stopped it before every model', async () => {
		stub.modelTestRows = [{ model_id: 'gpt-4o', ok: true, latency_ms: 30 }];
		stub.modelTestTotal = 9;
		stub.modelTestStopped = 'deadline';
		await renderCatalog();

		await fireEvent.click(sweepButton());

		await waitFor(() => {
			expect(squashed(screen.getByText(/Tested 1 of 9 models/))).toContain(
				'The sweep ran out of time before every model.'
			);
		});
	});

	it('renders the sentence the server sent when it refuses the sweep', async () => {
		stub.modelTestStatus = 400;
		await renderCatalog();

		await fireEvent.click(sweepButton());

		await waitFor(() => {
			expect(squashed(screen.getByRole('alert'))).toBe(
				'This provider offers no chat model to test.'
			);
		});
	});

	it('drops the previous answers when the catalog is read again', async () => {
		stub.modelTestRows = [{ model_id: 'gpt-4o', name: 'GPT-4o', ok: true, latency_ms: 30 }];
		const container = await renderCatalog();

		await fireEvent.click(testButton(rowOf(container, 'gpt-4o')));
		await waitFor(() => {
			expect(squashed(within(rowOf(container, 'gpt-4o')).getByText('Answered in 30 ms'))).toBe(
				'Answered in 30 ms'
			);
		});

		view.rerender({ token: 1 });
		await waitFor(() => {
			expect(squashed(within(rowOf(container, 'gpt-4o')).getByText('Not tested'))).toBe(
				'Not tested'
			);
		});
	});
});
