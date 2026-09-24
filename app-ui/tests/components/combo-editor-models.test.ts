// The combo editor's model list (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// Two rules about the list itself, split out of `combo-editor.test.ts` by concern: the list is an ORDER,
// so a move rewrites the priorities, and the picker beside it toggles membership of that same ordered
// list while a judge pick replaces one value and closes.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ComboEditor from '../../src/lib/components/ComboEditor.svelte';
import { schemaCombo, type Combo } from '$lib/schemas/combo';
import type { PickerSection } from '$lib/schemas/model-picker';

// The picker's own fixture: one active provider, one ref, and a combo name.
const sections: PickerSection[] = [
	{ key: 'combos', label: 'Combos', options: [{ value: 'daily', label: 'daily' }] },
	{ key: 'openai', label: 'OpenAI', options: [{ value: 'openai/gpt-4o', label: 'GPT-4o' }] }
];

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

describe('the combo editor model list', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renumbers priorities when a model is moved up the list', async () => {
		const { calls } = stubFetch(200);
		const existing = combo({
			models: [
				{ ref: 'first/m', priority: 0 },
				{ ref: 'second/m', priority: 1 }
			]
		});

		render(ComboEditor, {
			props: {
				combo: existing,
				sections: [],
				pickerLoading: false,
				pickerFailed: false,
				onsaved: vi.fn(),
				oncancel: vi.fn()
			}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Move second/m up' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save the combo' }));

		await waitFor(() => expect(calls).toHaveLength(1));
		expect(calls[0].body).toMatchObject({
			models: [
				{ ref: 'second/m', priority: 0 },
				{ ref: 'first/m', priority: 1 }
			]
		});
	});

	it('adds a member from the picker and takes it back out when the chip is clicked again', async () => {
		render(ComboEditor, {
			props: {
				combo: null,
				sections,
				pickerLoading: false,
				pickerFailed: false,
				onsaved: vi.fn(),
				oncancel: vi.fn()
			}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Add models' }));
		const dialog = await screen.findByRole('dialog');
		await fireEvent.click(within(dialog).getByRole('button', { name: 'GPT-4o' }));

		const refs = (): HTMLInputElement[] =>
			screen.getAllByLabelText('Model reference') as HTMLInputElement[];
		expect(refs().map((input) => input.value)).toEqual(['', 'openai/gpt-4o']);

		await fireEvent.click(within(dialog).getByRole('button', { name: 'GPT-4o' }));
		expect(refs().map((input) => input.value)).toEqual(['']);
	});

	it('fills the judge from the picker in single-select mode and closes it', async () => {
		render(ComboEditor, {
			props: {
				combo: null,
				sections,
				pickerLoading: false,
				pickerFailed: false,
				onsaved: vi.fn(),
				oncancel: vi.fn()
			}
		});

		await selectStrategy('fusion');
		await fireEvent.click(screen.getByRole('button', { name: 'Choose' }));
		const dialog = await screen.findByRole('dialog');
		await fireEvent.click(within(dialog).getByRole('button', { name: 'GPT-4o' }));

		expect((screen.getByLabelText('Judge model') as HTMLInputElement).value).toBe('openai/gpt-4o');
		expect(document.querySelector('dialog')?.hasAttribute('open')).toBe(false);
	});
});
