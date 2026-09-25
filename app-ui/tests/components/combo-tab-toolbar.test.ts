// The Combos tab's toolbar and pagination (owner directive, 2026-09-25; docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The owner asked for a compact, symmetric layout on this screen. The measurable half of that is the
// toolbar: the create action and the refresh control share one row with the sentence, rather than each
// taking a line of its own, and every control in the row is one height. The other measurable half is the
// pagination: Previous and Next carry the same chevrons the provider list uses, so the two screens cannot
// drift apart on the same control.
//
// What a unit test can hold here is the structure (one row, one glyph each) and the accessible names. The
// baseline measurement is a live click-through, which the port document records.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import CombosTab from '../../src/lib/components/CombosTab.svelte';

const combo = {
	id: 'cmb_1',
	name: 'daily',
	strategy: 'fallback',
	sticky_limit: 1,
	judge_model: '',
	models: [{ ref: 'openai/gpt-4o', priority: 0 }],
	created_at: '2026-09-18T00:00:00Z',
	updated_at: '2026-09-18T00:00:00Z'
};

// Serves the tab's three reads and records the combo page it was asked for, so paging is asserted on the
// wire rather than on the label alone.
function stubFetch(options: { total?: number; fail?: boolean } = {}): string[] {
	const requested: string[] = [];
	const total = options.total ?? 1;

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		requested.push(url);
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
			if (options.fail === true) {
				return json(
					{ error: { code: 'INTERNAL_ERROR', message: 'The combo list could not be read.' } },
					500
				);
			}
			return json({ data: [combo], meta: { page: 1, per_page: 25, total } });
		}
		return json({ error: { code: 'NOT_FOUND', message: 'No route matches that request.' } }, 404);
	});

	return requested;
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

// An empty combo page, which is what reaches the tab's empty state. Kept apart from `stubFetch` because
// that one always answers a row, so the empty state would never be reached through it.
function stubEmpty(): void {
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
		if (url.includes('/combos'))
			return json({ data: [], meta: { page: 1, per_page: 25, total: 0 } });
		return json({ error: { code: 'NOT_FOUND', message: 'No route matches that request.' } });
	});
}

describe('the combos toolbar', () => {
	it('puts the sentence, the create action, and the refresh control on one row', async () => {
		stubFetch();
		render(CombosTab);

		const create = await screen.findByRole('button', { name: 'New combo' });
		const refresh = screen.getByRole('button', { name: 'Refresh now' });
		const sentence = screen.getByText(
			'A combo is a model string that resolves to several upstream models.'
		);

		// One row means one parent: the sentence's block and the controls' block are siblings inside the
		// same flex row, so the controls cannot be pushed onto a line of their own by the copy above them.
		const row = sentence.parentElement;
		expect(row?.className).toContain('flex-wrap');
		expect(row?.contains(create)).toBe(true);
		expect(row?.contains(refresh)).toBe(true);

		// Every control in the row is one height, which is what makes the row read as a row.
		for (const control of [create, refresh]) {
			expect(control.className, `${control.textContent?.trim()} is not one height`).toContain(
				'min-h-11'
			);
		}
	});

	it('gives the create action and the refresh control a glyph beside the label', async () => {
		stubFetch();
		render(CombosTab);

		for (const name of ['New combo', 'Refresh now']) {
			const button = await screen.findByRole('button', { name });
			expect(button.querySelector('svg'), `${name} has no glyph`).toBeTruthy();
			expect(button.textContent?.trim(), `${name} lost its label`).not.toBe('');
		}
	});
});

describe('the combos pagination', () => {
	it('renders Previous and Next with the chevrons the provider list uses', async () => {
		stubFetch({ total: 60 });
		render(CombosTab);

		for (const name of ['Previous', 'Next']) {
			const button = await screen.findByRole('button', { name });
			expect(button.querySelector('svg'), `${name} has no glyph`).toBeTruthy();
			expect(button.className).toContain('min-h-11');
		}
	});

	it('disables Previous on the first page and asks for the next one', async () => {
		const requested = stubFetch({ total: 60 });
		render(CombosTab);

		const previous = (await screen.findByRole('button', { name: 'Previous' })) as HTMLButtonElement;
		const next = screen.getByRole('button', { name: 'Next' }) as HTMLButtonElement;

		expect(previous.disabled).toBe(true);
		expect(next.disabled).toBe(false);

		await fireEvent.click(next);
		await waitFor(() =>
			expect(requested.some((url) => url.includes('/combos') && url.includes('page=2'))).toBe(true)
		);
	});
});

describe('the combos states', () => {
	it('renders Try again with a glyph when the read fails', async () => {
		stubFetch({ fail: true });
		render(CombosTab);

		const retry = await screen.findByRole('button', { name: 'Try again' });
		expect(retry.querySelector('svg'), 'Try again has no glyph').toBeTruthy();
	});

	it('renders the empty-state create action with a glyph', async () => {
		stubEmpty();
		render(CombosTab);

		const create = await screen.findByRole('button', { name: 'Create the first combo' });
		expect(create.querySelector('svg'), 'Create the first combo has no glyph').toBeTruthy();
	});
});
