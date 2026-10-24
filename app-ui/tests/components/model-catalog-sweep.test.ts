// The bounded catalog sweep (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A sweep fills exactly the rows it probed and says how many it did not, so the budget is asserted from the
// button's own sentence and the coverage from the server's answer. The single-row probe is
// `model-catalog-probe.test.ts`.

import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { squashed } from '../support/dom';
import { MODEL_TEST_SWEEP_LIMIT } from '$lib/schemas/model-test';
import {
	probeFixture,
	renderCatalog,
	rerenderCatalog,
	rowOf,
	testButton,
	type ProbeStore
} from '../support/model-catalog-probe-harness';
import type { ModelStub } from '../support/model-stub';

let stub: ModelStub;
let tests: ProbeStore;

beforeEach(() => {
	({ stub, tests } = probeFixture());
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
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
		await renderCatalog(tests);

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
		const container = await renderCatalog(tests);

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
		await renderCatalog(tests);

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
		await renderCatalog(tests);

		await fireEvent.click(sweepButton());

		await waitFor(() => {
			expect(squashed(screen.getByText(/Tested 1 of 9 models/))).toContain(
				'The sweep ran out of time before every model.'
			);
		});
	});

	it('renders the sentence the server sent when it refuses the sweep', async () => {
		stub.modelTestStatus = 400;
		await renderCatalog(tests);

		await fireEvent.click(sweepButton());

		await waitFor(() => {
			expect(squashed(screen.getByRole('alert'))).toBe(
				'This provider offers no chat model to test.'
			);
		});
	});

	it('drops the previous answers when the catalog is read again', async () => {
		stub.modelTestRows = [{ model_id: 'gpt-4o', name: 'GPT-4o', ok: true, latency_ms: 30 }];
		const container = await renderCatalog(tests);

		await fireEvent.click(testButton(rowOf(container, 'gpt-4o')));
		await waitFor(() => {
			expect(squashed(within(rowOf(container, 'gpt-4o')).getByText('Answered in 30 ms'))).toBe(
				'Answered in 30 ms'
			);
		});

		rerenderCatalog({ token: 1 });
		await waitFor(() => {
			expect(squashed(within(rowOf(container, 'gpt-4o')).getByText('Not tested'))).toBe(
				'Not tested'
			);
		});
	});
});
