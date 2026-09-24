// The §8.4.4 wiring on the combo editor, the sixth form that holds an unsaved draft.
//
// Split from `tests/components/dirty-forms.test.ts` when Prettier reflowed that file past the 220-line
// warning, on the seam that matters: this is the one form whose dirty state is a field-by-field comparison
// of the seeded form (`comboFormDirty` in `src/lib/schemas/combo-form.ts`) rather than a comparison against
// a settings document. Both of its cases are here, and they are the pair that makes the baseline worth
// testing: a stored combo being edited, and the editor in create mode, where the seeded form is empty and
// the first keystroke is the draft. A baseline that moved with the draft would leave both reading clean.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, vi } from 'vitest';
import ComboEditor from '../../src/lib/components/ComboEditor.svelte';
import { hasDirtyForm } from '../../src/lib/dirty-guard';
import { schemaCombo } from '$lib/schemas/combo';
import { forEachCase } from '../support/tables';

function comboFixture() {
	return schemaCombo.parse({
		id: 'cmb_1',
		name: 'daily',
		strategy: 'fallback',
		sticky_limit: 1,
		judge_model: '',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		created_at: '2026-09-18T00:00:00Z',
		updated_at: '2026-09-18T00:00:00Z'
	});
}

type FormCase = {
	name: string;
	/** Renders the form and waits until its draft is seeded, which the guard must see as clean. */
	mount: () => Promise<void>;
	/** One edit, the smallest change the operator can make. */
	edit: () => Promise<void>;
	/** The same edit undone, so the form is back to what it loaded. */
	undo: () => Promise<void>;
};

const CASES: FormCase[] = [
	{
		name: 'the Combo editor editing a stored combo',
		mount: async () => {
			render(ComboEditor, {
				props: {
					combo: comboFixture(),
					sections: [],
					pickerLoading: false,
					pickerFailed: false,
					onsaved: vi.fn(),
					oncancel: vi.fn()
				}
			});
			await screen.findByRole('heading', { name: 'Edit daily' });
		},
		edit: async () => {
			await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'daily-2' } });
		},
		undo: async () => {
			await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'daily' } });
		}
	},
	{
		name: 'the Combo editor in create mode',
		mount: async () => {
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
			await screen.findByRole('heading', { name: 'New combo' });
		},
		edit: async () => {
			await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'evening' } });
		},
		undo: async () => {
			await fireEvent.input(screen.getByLabelText('Name'), { target: { value: '' } });
		}
	}
];

describe('the combo editor draft', () => {
	afterEach(() => {
		cleanup();
	});

	forEachCase(CASES, async (testCase) => {
		await testCase.mount();

		// A form that has only loaded is not a draft: the guard must stay quiet until the operator edits.
		expect(hasDirtyForm()).toBe(false);

		await testCase.edit();
		await waitFor(() => expect(hasDirtyForm()).toBe(true));

		await testCase.undo();
		await waitFor(() => expect(hasDirtyForm()).toBe(false));
	});
});
