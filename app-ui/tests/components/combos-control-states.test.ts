// Every control on the Combos screen carries the states (DESIGN.md §3.5).
//
// Twenty-four controls here were hand-written, in three different readings of the same affordance: the
// create action was an accent fill wearing a warm hairline border, the picker's chosen chips were accent
// fills too, and the row actions washed to a raised surface with no press under any of them. This screen
// is also where a chosen control needed a step of its own, because its chips are selected rather than
// hovered, and where a text action shipped with no touch target at all.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import CombosTab from '../../src/lib/components/CombosTab.svelte';
import ModelPickerDialog from '../../src/lib/components/ModelPickerDialog.svelte';
import { expectEveryControlHasStates } from '../support/control-states';

const SECTIONS = [
	{
		key: 'connected',
		label: 'Connected',
		options: [
			{ value: 'openai/gpt-4o', label: 'GPT-4o' },
			{ value: 'anthropic/claude', label: 'Claude', placeholder: true }
		]
	}
];

function stubCombos(): void {
	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		const json = (payload: unknown): Response =>
			new Response(JSON.stringify(payload), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});

		if (url.includes('/models/catalog')) return json({ data: [] });
		if (url.includes('/providers'))
			return json({ data: [], meta: { page: 1, per_page: 100, total: 0 } });
		return json({
			data: [
				{
					id: 'cmb_1',
					name: 'research',
					strategy: 'fallback',
					judge_model: '',
					models: [{ ref: 'openai/gpt-4o', priority: 0 }],
					created_at: '2026-09-18T00:00:00Z',
					updated_at: '2026-09-18T00:00:00Z'
				}
			],
			meta: { page: 1, per_page: 25, total: 1 }
		});
	});
}

function everyControl(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button'));
}

describe('Combos control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives the list, its toolbar, its row actions and its paging the same states', async () => {
		stubCombos();
		render(CombosTab);
		await screen.findByRole('button', { name: 'New combo' });

		expectEveryControlHasStates(everyControl(), 'Combos list');
	});

	it('gives the editor, its model rows, and the judge control the same', async () => {
		stubCombos();
		render(CombosTab);
		await screen.findByRole('button', { name: 'New combo' });

		await fireEvent.click(screen.getByRole('button', { name: 'New combo' }));
		await screen.findByLabelText('Name');

		const controls = everyControl();
		expect(controls.length, 'the editor rendered no controls').toBeGreaterThan(4);
		expectEveryControlHasStates(controls, 'Combo editor');
	});

	it('marks a chosen model chip on the pressed surface rather than with a second accent', async () => {
		const onclose = vi.fn();
		render(ModelPickerDialog, {
			props: {
				title: 'Add models',
				open: true,
				sections: SECTIONS,
				selected: ['openai/gpt-4o'],
				emptyText: 'No connected provider offers a model yet.',
				ontoggle: () => {},
				onclose
			}
		});

		const chosen = screen.getByRole('button', { name: 'GPT-4o' });
		expect(chosen.getAttribute('aria-pressed')).toBe('true');
		expect(chosen.className).toContain('aria-[pressed=true]:bg-[var(--color-surface-3)]');
		expect(chosen.className).not.toContain('bg-[var(--color-accent)]');

		// The placeholder option keeps its own shape mark and shares the chosen and hover readings.
		const placeholder = screen.getByRole('button', { name: 'Claude' });
		expect(placeholder.className).toContain('border-dashed');
		expectEveryControlHasStates(everyControl(), 'Model picker');
	});
});
