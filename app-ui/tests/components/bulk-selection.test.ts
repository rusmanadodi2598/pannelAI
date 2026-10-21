// The bulk action bar and the endpoint table's selection (docs/SPEC-UI/001-SPEC-UI.md §6.2, §8.11.9).
//
// Two promises are tested here and neither is cosmetic. The first is the owner's rule that these controls
// are icon-only: no word in the button, the accessible name in `aria-label`, the glyph decorative. The
// second is D9's: a Select-All speaks with the server's count, so a ten-row page of a forty-seven row set has
// to say forty-seven, and clearing has to name the number it is dropping. A bar that counted the rows on
// screen would look correct and be a lie.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import BulkActionBar from '$lib/components/BulkActionBar.svelte';
import EndpointTable from '$lib/components/EndpointTable.svelte';
import { expectIconOnly } from '../support/icon-only';
import { squashed } from '../support/dom';
import { rowChecked, toggleRow } from '$lib/utils/selection';
import { endpoint } from '../support/selection-fixtures';

afterEach(() => cleanup());

function renderBar(overrides: Record<string, unknown> = {}): void {
	render(BulkActionBar, {
		kind: 'endpoints',
		selected: 47,
		total: 47,
		scope: 'all 47 on the server',
		ondisable: vi.fn(),
		onenable: vi.fn(),
		ondelete: vi.fn(),
		onclear: vi.fn(),
		ontag: vi.fn(),
		ontoggleall: vi.fn(),
		...overrides
	});
}

describe('the bulk action bar', () => {
	it('renders every control icon-only, with the count in static text', () => {
		renderBar();

		const spoken = squashed(screen.getByTestId('bulk-count'));
		expect(spoken).toContain('47 endpoints selected');
		expect(spoken).toContain('all 47 on the server');

		for (const name of [
			'Select all 47 endpoints',
			'Tag 47 endpoints',
			'Disable 47 endpoints',
			'Enable 47 endpoints',
			'Delete 47 endpoints',
			'Clear selection of 47 endpoints'
		]) {
			const button = screen.getByRole('button', { name });
			expect(button.getAttribute('aria-label'), `${name} has no accessible name`).toBe(name);
			expectIconOnly(button, name);
		}
	});

	it('stays on screen with nothing selected, because that is when Select-All is needed', () => {
		renderBar({ selected: 0 });
		expect(screen.getByRole('button', { name: 'Select all 47 endpoints' })).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Delete 47 endpoints' })).toBeNull();
	});

	// The reported bug: the bar claimed the set while every row checkbox stayed empty.
	it('ticks the row checkboxes when Select-All is claimed from the bar', async () => {
		const onselect = vi.fn();
		const ontoggleall = vi.fn();
		renderBar({ selected: 0, onselect, ontoggleall });

		await fireEvent.click(screen.getByRole('button', { name: 'Select all 47 endpoints' }));
		expect(ontoggleall).toHaveBeenCalledWith(true);
	});

	// The duplicate was a second control named and drawn exactly like the Select-All toggle, which
	// had already flipped to "Clear selection" under a whole-set claim.
	it('offers one clear control, never two, in every selection state', () => {
		// Each state is its own mount: without the unmount the three toolbars stack up in one
		// document and every count below reads as double.
		const clear = (overrides: Record<string, unknown>) => {
			cleanup();
			renderBar(overrides);
			return screen.queryAllByRole('button', { name: /Clear selection/ }).length;
		};

		expect(clear({ selected: 0 })).toBe(0);
		expect(clear({ selected: 3 })).toBe(1);
		expect(clear({ selected: 47, allSelected: true })).toBe(1);
		expect(
			screen.queryAllByRole('button', { name: 'Select all 47 endpoints' }).length,
			'the toggle stays one control after it flips to clearing'
		).toBe(0);
	});

	it('renders every loaded row checked while the whole set is claimed', () => {
		render(EndpointTable, {
			endpoints: [
				endpoint({ id: 'ep_1', label: 'primary' }),
				endpoint({ id: 'ep_2', label: 'spare' })
			],
			onopen: vi.fn(),
			selected: [],
			allSelected: true,
			onselect: vi.fn()
		});

		for (const name of ['Select primary for bulk actions', 'Select spare for bulk actions']) {
			expect((screen.getByRole('checkbox', { name }) as HTMLInputElement).checked).toBe(true);
		}
	});

	it('drops the tag control where rows carry no tags', () => {
		renderBar({ ontag: undefined });
		expect(screen.queryByRole('button', { name: 'Tag 47 endpoints' })).toBeNull();
		expect(screen.getByRole('button', { name: 'Disable 47 endpoints' })).toBeTruthy();
	});

	it('routes each control to the action it names', async () => {
		const ondisable = vi.fn();
		const onclear = vi.fn();
		renderBar({ ondisable, onclear });

		await fireEvent.click(screen.getByRole('button', { name: 'Disable 47 endpoints' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Clear selection of 47 endpoints' }));

		expect(ondisable).toHaveBeenCalledTimes(1);
		expect(onclear).toHaveBeenCalledTimes(1);
	});
});

describe('the endpoint table selection', () => {
	// Select-All is not a table header control: `src/app.css` clips the whole `<thead>` below 768px, so a
	// header checkbox would simply not exist on a phone. It lives in the bar, asserted above.
	it('keeps the row checkbox inside the Actions cell, beside the other actions', async () => {
		const onselect = vi.fn();
		render(EndpointTable, {
			endpoints: [endpoint({ id: 'ep_1', label: 'primary' })],
			onopen: vi.fn(),
			ondelete: vi.fn(),
			selected: [],
			onselect
		});

		// The delete control is icon-only, so it is found by its name rather than by text.
		const deleteButton = screen.getByRole('button', { name: 'Delete' });
		const box = screen.getByRole('checkbox', { name: 'Select primary for bulk actions' });
		expect(
			box.closest('td')?.contains(deleteButton),
			'row checkbox and the delete action are not the same cell'
		).toBe(true);

		await fireEvent.click(box);
		expect(onselect).toHaveBeenCalledWith('ep_1', true);
	});

	// Two inline-block controls each sitting on their own baseline is how the checkbox ended up a few
	// pixels off the delete glyph. One flex line with one target size is the machine-checkable form.
	it('centres the row checkbox on the same line as the delete glyph', () => {
		render(EndpointTable, {
			endpoints: [endpoint({ id: 'ep_1', label: 'primary' })],
			onopen: vi.fn(),
			ondelete: vi.fn(),
			selected: ['ep_1'],
			onselect: vi.fn()
		});

		const box = screen.getByRole('checkbox', { name: 'Select primary for bulk actions' });
		const deleteButton = screen.getByRole('button', { name: 'Delete' });
		const group = box.closest('div');

		expect(group?.contains(deleteButton), 'checkbox and the delete glyph are not one group').toBe(
			true
		);
		expect(group?.className).toContain('items-center');
		// The 44px square now comes from the primitive's icon size rather than a height written at the call
		// site: the same measurement, with one owner.
		expect(deleteButton.className).toContain('size-11');
		expectIconOnly(deleteButton, 'Delete');
	});

	it('renders tags as chips in the identity cell, and parks in words not colour', () => {
		render(EndpointTable, {
			endpoints: [
				endpoint({
					id: 'ep_1',
					label: 'primary',
					status: 'disabled',
					tags: ['free-tier', 'event-week'],
					disabled_until: '2026-10-17T12:00:00Z'
				})
			],
			onopen: vi.fn(),
			selected: [],
			onselect: vi.fn()
		});

		expect(screen.getByText('free-tier')).toBeTruthy();
		expect(screen.getByText('event-week')).toBeTruthy();
		expect(screen.getByText('until', { exact: false }).textContent).toContain('until');
	});

	// "all except this one" is not a selector the API accepts, so taking one row out of a
	// whole-set claim falls back to the ids the page holds, and the bar says so.
	it('falls back to the loaded ids when one row leaves a whole-set claim', () => {
		const next = toggleRow({ ids: [], all: true }, 'ep_2', false, ['ep_1', 'ep_2']);
		expect(next).toEqual({ ids: ['ep_1'], all: false });
		expect(rowChecked(next, 'ep_1')).toBe(true);
		expect(rowChecked(next, 'ep_2')).toBe(false);
	});

	it('renders no checkbox column when the caller cannot act on a set', () => {
		render(EndpointTable, { endpoints: [endpoint()], onopen: vi.fn() });
		expect(screen.queryByRole('checkbox')).toBeNull();
	});
});
