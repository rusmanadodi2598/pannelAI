// Combo & Vision control icons (owner directive, 2026-09-25; docs/SPEC-UI/001-SPEC-UI.md §8.11).
//
// The contract is the same one the two Endpoint & Key tables and the provider screens hold
// (`tests/support/icon-only.ts`): when the visible text goes, the button must still carry the action's
// name as its accessible name, it must say so on hover, and the glyph must be decorative so a screen
// reader reads one thing rather than two. This file is the Combo & Vision surface's half of that
// contract, so a later edit that drops an `aria-label` fails here rather than shipping a button that
// looks right and is unusable.
//
// The destructive action's colour is part of the same contract: it must carry exactly one `text-*`
// utility, the danger one, because two competing utilities resolve by stylesheet order and the muted one
// silently won the Delete button's colour on the Endpoint & Key table until it was measured live
// (2026-09-24). jsdom has no Tailwind cascade, so the class list is the only part of that rule a unit
// test can hold.
//
// The toolbar controls keep their visible labels and gain the glyph beside them (SPEC-UI §8.11 butir 10),
// so they are asserted the other way round: the label stays, and the glyph rides along.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ComboEditor from '../../src/lib/components/ComboEditor.svelte';
import ComboTable from '../../src/lib/components/ComboTable.svelte';
import { schemaCombo, type Combo } from '$lib/schemas/combo';
import type { PickerSection } from '$lib/schemas/model-picker';
import { expectIconOnly } from '../support/icon-only';

const combo = (overrides: Record<string, unknown> = {}): Combo =>
	schemaCombo.parse({
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

const sections: PickerSection[] = [
	{ key: 'openai', label: 'OpenAI', options: [{ value: 'openai/gpt-4o', label: 'GPT-4o' }] }
];

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('combo table row actions', () => {
	function row(): HTMLElement {
		render(ComboTable, {
			props: {
				combos: [combo()],
				deleting: null,
				onedit: () => {},
				ondelete: () => {}
			}
		});

		return screen.getByText('daily').closest('tr') as HTMLElement;
	}

	// A table of the three actions, so a fourth added later has to state its own contract rather than
	// inheriting one of these by accident.
	const ACTIONS = ['Edit', 'Test', 'Delete'] as const;

	it.each(ACTIONS)('renders %s as a named icon', (name) => {
		expectIconOnly(within(row()).getByRole('button', { name }), name);
	});

	it('gives the destructive action the danger colour, with no competing text colour', () => {
		const button = within(row()).getByRole('button', { name: 'Delete' });

		expect(button.className).toContain('text-[var(--color-danger)]');
		expect(button.className).not.toContain('text-[var(--color-text-muted)]');
	});
});

describe('the combo editor model rows', () => {
	function renderEditor(models: { ref: string; priority: number }[]): void {
		render(ComboEditor, {
			props: {
				combo: combo({ models }),
				sections,
				pickerLoading: false,
				pickerFailed: false,
				onsaved: () => {},
				oncancel: () => {}
			}
		});
	}

	// Two rows, so the middle entry has both moves enabled and the ends have one disabled each. A single
	// row would leave both buttons disabled and the assertion would pass on a control nobody can press.
	const MOVES = [
		{ label: 'Move openai/gpt-4o up', disabled: false },
		{ label: 'Move openai/gpt-4o down', disabled: false },
		{ label: 'Remove openai/gpt-4o', disabled: false }
	] as const;

	it.each(MOVES)('renders $label as a named icon', ({ label }) => {
		renderEditor([
			{ ref: 'openai/gpt-4o', priority: 0 },
			{ ref: 'openai/o3-mini', priority: 1 }
		]);

		expectIconOnly(screen.getAllByRole('button', { name: label })[0], label);
	});

	it('keeps the first entry from moving up and the last from moving down', () => {
		renderEditor([
			{ ref: 'openai/gpt-4o', priority: 0 },
			{ ref: 'openai/o3-mini', priority: 1 }
		]);

		const up = screen.getAllByRole('button', { name: /Move .* up/ }) as HTMLButtonElement[];
		const down = screen.getAllByRole('button', { name: /Move .* down/ }) as HTMLButtonElement[];

		expect(up.map((button) => button.disabled)).toEqual([true, false]);
		expect(down.map((button) => button.disabled)).toEqual([false, true]);
	});

	it('gives the remove action the danger colour, with no competing text colour', () => {
		renderEditor([{ ref: 'openai/gpt-4o', priority: 0 }]);

		const button = screen.getByRole('button', { name: 'Remove openai/gpt-4o' });
		expect(button.className).toContain('text-[var(--color-danger)]');
		expect(button.className).not.toContain('text-[var(--color-text-muted)]');
	});
});
