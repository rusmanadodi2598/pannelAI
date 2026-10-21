// What the combo editor sends, and what a refusal leaves on screen (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The body must carry only the fields the chosen strategy reads, because a leftover judge model on a
// fallback combo is a request the API refuses. The hidden-field rule that goes with it is
// `combo-editor.test.ts`; the model list and its picker are `combo-editor-models.test.ts`.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ComboEditor from '../../src/lib/components/ComboEditor.svelte';
import { combo, selectStrategy, stubFetch } from '../support/combo-editor-harness';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('ComboEditor save', () => {
	it('sends only the fields a fallback combo reads', async () => {
		const { calls } = stubFetch();
		const onsaved = vi.fn();
		render(ComboEditor, {
			props: {
				combo: null,
				sections: [],
				pickerLoading: false,
				pickerFailed: false,
				onsaved,
				oncancel: vi.fn()
			}
		});

		await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'daily' } });
		await fireEvent.input(screen.getByLabelText('Model reference'), {
			target: { value: 'openai/gpt-4o' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Create the combo' }));

		await waitFor(() => expect(calls).toHaveLength(1));
		expect(calls[0].url).toContain('/api/v1/combos');
		expect(calls[0].body).toEqual({
			name: 'daily',
			strategy: 'fallback',
			models: [{ ref: 'openai/gpt-4o', priority: 0 }]
		});
		// The request is recorded before the response is handled, so waiting on the call alone would assert
		// against a save that had not finished.
		await waitFor(() => expect(onsaved).toHaveBeenCalledTimes(1));
	});

	it('drops a judge model that was typed before the strategy changed away from fusion', async () => {
		const { calls } = stubFetch();
		render(ComboEditor, {
			props: {
				combo: null,
				sections: [],
				pickerLoading: false,
				pickerFailed: false,
				onsaved: vi.fn(),
				oncancel: vi.fn()
			}
		});

		await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'mixed' } });
		await fireEvent.input(screen.getByLabelText('Model reference'), { target: { value: 'a/b' } });
		await selectStrategy('fusion');
		await fireEvent.input(screen.getByLabelText('Judge model'), { target: { value: 'j/m' } });
		await selectStrategy('fallback');
		await fireEvent.click(screen.getByRole('button', { name: 'Create the combo' }));

		await waitFor(() => expect(calls).toHaveLength(1));
		expect(calls[0].body).toEqual({
			name: 'mixed',
			strategy: 'fallback',
			models: [{ ref: 'a/b', priority: 0 }]
		});
		expect(calls[0].body).not.toHaveProperty('judge_model');
	});

	it('reports a refused save rather than closing the editor', async () => {
		stubFetch(409);
		const onsaved = vi.fn();

		vi.stubGlobal(
			'fetch',
			async () =>
				new Response(
					JSON.stringify({ error: { code: 'CONFLICT', message: 'That name is taken.' } }),
					{
						status: 409,
						headers: { 'content-type': 'application/json' }
					}
				)
		);

		render(ComboEditor, {
			props: {
				combo: null,
				sections: [],
				pickerLoading: false,
				pickerFailed: false,
				onsaved,
				oncancel: vi.fn()
			}
		});

		await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'daily' } });
		await fireEvent.input(screen.getByLabelText('Model reference'), { target: { value: 'a/b' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Create the combo' }));

		expect(await screen.findByText('That name is taken.')).toBeTruthy();
		expect(onsaved).not.toHaveBeenCalled();
	});

	it('refills the form when it is handed a different combo', async () => {
		const { rerender } = render(ComboEditor, {
			props: {
				combo: combo(),
				sections: [],
				pickerLoading: false,
				pickerFailed: false,
				onsaved: vi.fn(),
				oncancel: vi.fn()
			}
		});

		expect((screen.getByLabelText('Name') as HTMLInputElement).value).toBe('daily');

		await rerender({
			combo: combo({ id: 'cmb_2', name: 'nightly', strategy: 'round_robin', sticky_limit: 3 }),
			sections: [],
			pickerLoading: false,
			pickerFailed: false,
			onsaved: vi.fn(),
			oncancel: vi.fn()
		});

		await waitFor(() => {
			expect((screen.getByLabelText('Name') as HTMLInputElement).value).toBe('nightly');
		});
		expect((screen.getByLabelText('Sticky limit') as HTMLInputElement).value).toBe('3');
	});
});
