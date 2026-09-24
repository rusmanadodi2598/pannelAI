// Combo editor component tests (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// Two rules are worth asserting against the real form rather than against a helper. The first is that the
// field a strategy ignores is hidden, not disabled, so the panel never shows a control that cannot affect
// the save. The second is that the body the panel actually sends carries only the fields the chosen
// strategy reads, because a leftover judge model on a fallback combo is a request the API refuses. The
// model list and the picker beside it have their own file (`combo-editor-models.test.ts`).

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ComboEditor from '../../src/lib/components/ComboEditor.svelte';
import { schemaCombo, type Combo } from '$lib/schemas/combo';

function combo(overrides: Record<string, unknown> = {}): Combo {
	return schemaCombo.parse({
		id: 'cmb_1',
		name: 'daily',
		strategy: 'fallback',
		sticky_limit: 1,
		judge_model: '',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		created_at: '2026-09-18T00:00:00Z',
		updated_at: '2026-09-18T00:00:00Z',
		...overrides
	});
}

/** Records every request the editor makes, so a test can assert what was sent as well as what was shown. */
function stubFetch(status = 201): { calls: { url: string; body: unknown }[] } {
	const calls: { url: string; body: unknown }[] = [];

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		calls.push({
			url: String(input),
			body: init?.body === undefined ? undefined : JSON.parse(String(init.body))
		});
		return new Response(JSON.stringify(combo()), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return { calls };
}

async function selectStrategy(value: string): Promise<void> {
	await fireEvent.change(screen.getByLabelText('Strategy'), { target: { value } });
}

describe('ComboEditor', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('hides the sticky limit unless the strategy is round_robin', async () => {
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

		expect(screen.queryByLabelText('Sticky limit')).toBeNull();

		await selectStrategy('round_robin');
		expect(screen.getByLabelText('Sticky limit')).toBeTruthy();

		await selectStrategy('fusion');
		expect(screen.queryByLabelText('Sticky limit')).toBeNull();
	});

	it('hides the judge model unless the strategy is fusion', async () => {
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

		expect(screen.queryByLabelText('Judge model')).toBeNull();

		await selectStrategy('round_robin');
		expect(screen.queryByLabelText('Judge model')).toBeNull();

		await selectStrategy('fusion');
		expect(screen.getByLabelText('Judge model')).toBeTruthy();
	});

	it("explains the selected strategy in the router's own words", async () => {
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

		expect(screen.getByText('Try models in order until one succeeds.')).toBeTruthy();

		await selectStrategy('round_robin');
		expect(
			screen.getByText(
				'Distribute across models, keeping sticky_limit requests on one model first.'
			)
		).toBeTruthy();
	});

	it('refuses a fusion combo with no judge model and sends nothing', async () => {
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
		await selectStrategy('fusion');
		await fireEvent.click(screen.getByRole('button', { name: 'Create the combo' }));

		expect(await screen.findByText(/needs a judge model/)).toBeTruthy();
		expect(calls).toHaveLength(0);
	});

	it('refuses a round_robin combo with a sticky limit of zero and sends nothing', async () => {
		// The input carries no `min` attribute on purpose, so the browser does not block the submit and the
		// operator reads the panel's message rather than the browser's. This test is what holds that: with a
		// native bound the submit never fires and no message appears at all.
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
		await selectStrategy('round_robin');
		await fireEvent.input(screen.getByLabelText('Sticky limit'), { target: { value: '0' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Create the combo' }));

		expect(await screen.findByText(/sticky limit starts at 1/)).toBeTruthy();
		expect(calls).toHaveLength(0);
	});

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
