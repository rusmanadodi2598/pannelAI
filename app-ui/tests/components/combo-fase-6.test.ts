// Fase 6 Combo P0: touch targets and mobile reflow for the editor actions,
// the empty state, the judge row, and the pagination (F11, F16, F17, F18).
//
// Each table below pins a class contract, so a restyle that drops the mobile
// behavior fails here first. One `it` per finding holds its cases in an array,
// following TDD §2.5.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ComboEditor from '../../src/lib/components/ComboEditor.svelte';
import CombosTab from '../../src/lib/components/CombosTab.svelte';
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

function editorProps(comboValue: Combo | null) {
	return {
		combo: comboValue,
		sections: [],
		pickerLoading: false,
		pickerFailed: false,
		onsaved: vi.fn(),
		oncancel: vi.fn()
	};
}

// Serves the tab's three reads. The combo page carries `total` rows, so the
// pagination renders only when `total` exceeds one page.
function stubCombos(total: number): void {
	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		const json = (payload: unknown): Response =>
			new Response(JSON.stringify(payload), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});

		if (url.includes('/models/catalog')) return json({ data: [] });
		if (url.includes('/providers')) {
			return json({ data: [], meta: { page: 1, per_page: 100, total: 0 } });
		}
		if (url.includes('/combos')) {
			const rows = total === 0 ? [] : [combo()];
			return json({ data: rows, meta: { page: 1, per_page: 25, total } });
		}
		return json({ error: { code: 'NOT_FOUND', message: 'No route matches that request.' } });
	});
}

// Serves the tab's reads with a failed combos list, so the error state renders.
function stubCombosError(): void {
	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		const json = (payload: unknown, status = 200): Response =>
			new Response(JSON.stringify(payload), {
				status,
				headers: { 'content-type': 'application/json' }
			});

		if (url.includes('/models/catalog')) return json({ data: [] });
		if (url.includes('/providers')) {
			return json({ data: [], meta: { page: 1, per_page: 100, total: 0 } });
		}
		if (url.includes('/combos')) {
			return json({ error: { code: 'INTERNAL_ERROR', message: 'the list is unavailable' } }, 500);
		}
		return json({ error: { code: 'NOT_FOUND', message: 'No route matches that request.' } });
	});
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('F11 editor save and cancel', () => {
	it('keeps save and cancel symmetric, stacked on mobile, cancel outlined', () => {
		const modes = [
			{ comboValue: null, save: 'Create the combo' },
			{ comboValue: combo(), save: 'Save the combo' }
		];

		for (const mode of modes) {
			cleanup();
			render(ComboEditor, { props: editorProps(mode.comboValue) });

			const save = screen.getByRole('button', { name: mode.save });
			const cancel = screen.getByRole('button', { name: 'Cancel' });

			for (const token of ['min-h-12', 'px-4', 'w-full', 'justify-center', 'sm:w-auto']) {
				expect(save.className, `${mode.save} lost ${token}`).toContain(token);
				expect(cancel.className, `Cancel lost ${token} in ${mode.save} mode`).toContain(token);
			}
			expect(cancel.className, `Cancel is not outlined in ${mode.save} mode`).toContain('border');
			expect(
				save.parentElement?.className,
				`actions do not stack on mobile in ${mode.save} mode`
			).toContain('flex-col');
		}
	});
});

describe('F16 combos empty state', () => {
	it('gives the empty-state action a 44px target with horizontal padding', async () => {
		stubCombos(0);
		render(CombosTab);

		const create = await screen.findByRole('button', { name: 'Create the first combo' });
		// The padding is the shared button's px-3, not the px-2 this control used to write for itself.
		for (const token of ['min-h-11', 'px-3', 'inline-flex', 'items-center']) {
			expect(create.className, `empty-state action lost ${token}`).toContain(token);
		}
		expect(create.querySelector('svg'), 'empty-state action has no glyph').toBeTruthy();
	});

	it('gives the error-state retry a 44px target with horizontal padding', async () => {
		stubCombosError();
		render(CombosTab);

		const retry = await screen.findByRole('button', { name: 'Try again' });
		for (const token of ['min-h-11', 'px-3', 'inline-flex', 'items-center', 'underline']) {
			expect(retry.className, `error-state retry lost ${token}`).toContain(token);
		}
	});
});

describe('F17 judge row', () => {
	it('wraps the judge row and spans the input full width on mobile', async () => {
		render(ComboEditor, { props: editorProps(null) });

		await fireEvent.change(screen.getByLabelText('Strategy'), { target: { value: 'fusion' } });

		const input = screen.getByLabelText('Judge model');
		const row = input.parentElement;
		const choose = screen.getByRole('button', { name: 'Choose' });
		const cases = [
			{ node: row, token: 'flex-wrap', note: 'judge row does not wrap' },
			{ node: row, token: 'max-sm:flex-col', note: 'judge row does not collapse on mobile' },
			{
				node: row,
				token: 'max-sm:items-stretch',
				note: 'judge row does not stretch its children on mobile'
			},
			{ node: input, token: 'w-full', note: 'judge input is not full width on mobile' },
			{ node: input, token: 'min-w-0', note: 'judge input keeps a fixed floor that can overflow' },
			{ node: input, token: 'min-h-11', note: 'judge input lost its touch target' },
			{ node: choose, token: 'min-h-11', note: 'Choose lost its touch target' },
			{ node: choose, token: 'px-3', note: 'Choose lost its desktop padding' },
			{ node: choose, token: 'max-sm:w-full', note: 'Choose is not full width on mobile' },
			// Centered at every width now, which is the shared button's own rule.
			{ node: choose, token: 'justify-center', note: 'Choose is not centered' },
			{ node: choose, token: 'max-sm:min-h-12', note: 'Choose lost its mobile touch target' },
			{ node: choose, token: 'max-sm:px-4', note: 'Choose lost its mobile padding' }
		];

		for (const entry of cases) {
			expect(entry.node?.className ?? '', entry.note).toContain(entry.token);
		}
	});
});

describe('F18 combos pagination', () => {
	it('wraps the pagination row and pads both buttons', async () => {
		stubCombos(60);
		render(CombosTab);

		const previous = await screen.findByRole('button', { name: 'Previous' });
		const next = screen.getByRole('button', { name: 'Next' });

		expect(previous.parentElement?.className ?? '', 'pagination does not wrap').toContain(
			'flex-wrap'
		);
		for (const button of [previous, next]) {
			const name = button.textContent?.trim() ?? 'pagination button';
			for (const token of ['min-h-11', 'px-3']) {
				expect(button.className, `${name} lost ${token}`).toContain(token);
			}
		}
	});
});
