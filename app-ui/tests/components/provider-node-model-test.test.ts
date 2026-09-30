// Per-model test on a custom node's models (docs/SPEC-UI/001-SPEC-UI.md §6.3, draft 037 §12).
//
// A compatible node has no registry catalog: the rows its models section lists ARE the models it offers,
// which is exactly where an operator asks "does this one answer". These cases hold the action on the row,
// the sweep beside it, and the fact that both write the same store, so a sweep fills the rows the buttons
// then re-test one at a time.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderCustomModels from '$lib/components/ProviderCustomModels.svelte';
import { createModelTestStore } from '$lib/stores/model-test.svelte';
import { createProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';
import { customRow, stubModels, type ModelStub } from '../support/model-stub';
import { expectIconOnly } from '../support/icon-only';
import { squashed } from '../support/dom';

const NODE_ID = 'openai-compatible-01J';
const PREFIX = 'mycorp';

let stub: ModelStub;
let tests: ReturnType<typeof createModelTestStore>;

beforeEach(() => {
	stub = stubModels({
		providers: ['openai', NODE_ID],
		custom: [
			customRow({
				id: 'mdl_01',
				provider_id: NODE_ID,
				model_id: 'gpt-4o-mini',
				display_name: 'GPT-4o mini'
			}),
			customRow({
				id: 'mdl_02',
				provider_id: NODE_ID,
				model_id: 'deepseek-v3',
				display_name: 'DeepSeek V3'
			})
		]
	});
	tests = createModelTestStore();
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

async function renderSection(nodePrefix = PREFIX): Promise<void> {
	render(ProviderCustomModels, {
		props: {
			providerId: NODE_ID,
			thinking: createProviderThinkingStore(),
			nodePrefix,
			tests,
			onchanged: () => {}
		}
	});
	await screen.findByRole('table', { name: /custom models declared/i });
}

function rowOf(modelId: string): HTMLElement {
	const rows = [...document.querySelectorAll('tbody tr')];
	const match = rows.find((row) => row.textContent?.includes(modelId));
	if (!match) throw new Error(`no row for ${modelId}`);
	return match as HTMLElement;
}

function testButton(modelId: string): HTMLButtonElement {
	return within(rowOf(modelId)).getByRole('button', {
		name: /^Test/
	}) as HTMLButtonElement;
}

describe("a node's declared models", () => {
	it('offer a probe per row and name the budget of a sweep', async () => {
		await renderSection();

		expect(squashed(within(rowOf('gpt-4o-mini')).getByText('Not tested'))).toBe('Not tested');
		expect(testButton('deepseek-v3')).toBeTruthy();
		expect(
			screen.getByRole('button', { name: 'Test the first 6 models' }) as HTMLButtonElement
		).toBeTruthy();
	});

	it('keeps the probe button icon-only', async () => {
		await renderSection();
		expectIconOnly(testButton('gpt-4o-mini'), 'Test');
	});

	it('asks for the node and the model the row names', async () => {
		stub.modelTestRows = [
			{ model_id: 'gpt-4o-mini', name: 'GPT-4o mini', ok: true, latency_ms: 320 }
		];
		await renderSection();

		await fireEvent.click(testButton('gpt-4o-mini'));

		expect(stub.modelTests).toEqual([`${NODE_ID}/gpt-4o-mini`]);
		await waitFor(() => {
			expect(squashed(within(rowOf('gpt-4o-mini')).getByText('Answered in 320 ms'))).toBe(
				'Answered in 320 ms'
			);
		});
	});

	it('renders the gateway code and message on the row that failed', async () => {
		stub.modelTestRows = [
			{
				model_id: 'deepseek-v3',
				ok: false,
				latency_ms: 12,
				status: 401,
				error_code: 'UNAUTHORIZED',
				error: 'the key is revoked'
			}
		];
		await renderSection();

		await fireEvent.click(testButton('deepseek-v3'));

		await waitFor(() => {
			const alert = within(rowOf('deepseek-v3')).getByRole('alert');
			expect(squashed(alert)).toContain('Failed in 12 ms');
			expect(squashed(alert)).toContain('UNAUTHORIZED: the key is revoked');
		});
	});

	it('fills the rows from one sweep and states what it covered', async () => {
		stub.modelTestRows = [
			{ model_id: 'gpt-4o-mini', name: 'GPT-4o mini', ok: true, latency_ms: 210 },
			{
				model_id: 'deepseek-v3',
				name: 'DeepSeek V3',
				ok: false,
				latency_ms: 0,
				error_code: 'MODEL_TEST_TIMEOUT',
				error: 'the model did not answer within the probe budget'
			}
		];
		stub.modelTestTotal = 2;
		await renderSection();

		await fireEvent.click(screen.getByRole('button', { name: 'Test the first 6 models' }));

		expect(stub.modelSweeps).toEqual([{ provider: NODE_ID, limit: 6 }]);
		await waitFor(() => {
			expect(squashed(screen.getByText(/Tested 2 models/))).toBe(
				'Tested 2 models: 1 answered, 1 failed.'
			);
		});
		expect(squashed(within(rowOf('gpt-4o-mini')).getByText('Answered in 210 ms'))).toBe(
			'Answered in 210 ms'
		);
		const timedOut = within(rowOf('deepseek-v3')).getByRole('alert');
		expect(squashed(timedOut)).toContain('Failed');
		expect(squashed(timedOut)).toContain(
			'MODEL_TEST_TIMEOUT: the model did not answer within the probe budget'
		);
	});

	it('refuses to sweep a node whose upstream is unreachable, in the server words', async () => {
		stub.modelTestStatus = 400;
		await renderSection();

		await fireEvent.click(screen.getByRole('button', { name: 'Test the first 6 models' }));

		await waitFor(() => {
			expect(squashed(screen.getByRole('alert'))).toBe(
				'This provider offers no chat model to test.'
			);
		});
	});

	it('offers the same row action to a registry provider supplement, without a sweep', async () => {
		await renderSection('');

		expect(testButton('gpt-4o-mini')).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Test the first 6 models' })).toBeNull();
	});
});
