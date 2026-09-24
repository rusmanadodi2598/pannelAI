// Model picker dialog tests (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The dialog is deliberately dumb: it renders the sections it is handed and reports a click. These cases
// pin the behaviour a caller relies on (the toggle, the single-select close, the search reset) and the
// four sentences it must not blur together: a read still running, a failed read, an empty catalog, and a
// search that matched nothing.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ModelPickerDialog from '../../src/lib/components/ModelPickerDialog.svelte';
import type { PickerSection } from '$lib/schemas/model-picker';

const sections: PickerSection[] = [
	{ key: 'combos', label: 'Combos', options: [{ value: 'daily', label: 'daily' }] },
	{
		key: 'openai',
		label: 'OpenAI',
		options: [
			{ value: 'openai/gpt-4o', label: 'GPT-4o' },
			{ value: 'openai/o3-mini', label: 'o3 mini' }
		]
	},
	{
		key: 'th-1',
		label: 'TH HARBOR 1',
		options: [{ value: 'th-1/model-id', label: 'th-1/model-id', placeholder: true }]
	}
];

function renderDialog(overrides: Record<string, unknown> = {}) {
	const ontoggle = vi.fn();
	const onclose = vi.fn();
	const view = render(ModelPickerDialog, {
		props: {
			title: 'Add models',
			open: true,
			sections,
			selected: [],
			emptyText: 'No connected provider offers a model yet.',
			ontoggle,
			onclose,
			...overrides
		}
	});
	return { view, ontoggle, onclose };
}

afterEach(() => {
	cleanup();
});

describe('ModelPickerDialog', () => {
	it('renders every section with its count, the combos first', () => {
		renderDialog();

		expect(screen.getByText('Combos')).toBeTruthy();
		expect(screen.getByText('OpenAI')).toBeTruthy();
		expect(screen.getByText('TH HARBOR 1')).toBeTruthy();
		expect(screen.getByText('(2)')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'GPT-4o' })).toBeTruthy();
	});

	it('reports a click with the ref it stores, and marks the selected chips', async () => {
		const { ontoggle } = renderDialog({ selected: ['openai/gpt-4o'] });

		const picked = screen.getByRole('button', { name: 'GPT-4o' });
		expect(picked.getAttribute('aria-pressed')).toBe('true');
		expect(screen.getByRole('button', { name: 'o3 mini' }).getAttribute('aria-pressed')).toBe(
			'false'
		);

		await fireEvent.click(screen.getByRole('button', { name: 'o3 mini' }));
		expect(ontoggle).toHaveBeenCalledWith('openai/o3-mini');
	});

	it('filters by the search box and says when nothing matches', async () => {
		renderDialog();

		const search = screen.getByPlaceholderText('Search models');
		await fireEvent.input(search, { target: { value: 'o3' } });

		expect(screen.queryByRole('button', { name: 'GPT-4o' })).toBeNull();
		expect(screen.getByRole('button', { name: 'o3 mini' })).toBeTruthy();

		await fireEvent.input(search, { target: { value: 'nothing-like-this' } });
		expect(screen.getByText('No model matches this search.')).toBeTruthy();
	});

	it('marks a placeholder chip and explains it', () => {
		renderDialog();

		const placeholder = screen.getByRole('button', { name: 'th-1/model-id' });
		expect(placeholder.getAttribute('title')).toBe('Add it, then edit the model id in the editor.');
		expect(screen.getByText(/A dashed entry is a placeholder/)).toBeTruthy();
	});

	it('closes after one pick in single-select mode', async () => {
		const { ontoggle, onclose } = renderDialog({ single: true });

		await fireEvent.click(screen.getByRole('button', { name: 'GPT-4o' }));

		expect(ontoggle).toHaveBeenCalledWith('openai/gpt-4o');
		expect(onclose).toHaveBeenCalled();
	});

	it('does not close on a pick when several may be chosen', async () => {
		const { onclose } = renderDialog();

		await fireEvent.click(screen.getByRole('button', { name: 'GPT-4o' }));

		expect(onclose).not.toHaveBeenCalled();
	});

	it('says a failed read is a failed read, not an empty catalog', () => {
		renderDialog({ failed: true });

		expect(screen.getByRole('alert').textContent).toContain('could not be read');
		expect(screen.queryByRole('button', { name: 'GPT-4o' })).toBeNull();
	});

	it('says the read is still running instead of claiming nothing is offered', async () => {
		const { view } = renderDialog({ sections: [], loading: true });

		expect(screen.getByText('Loading the catalog and the provider list.')).toBeTruthy();
		expect(screen.queryByText('No connected provider offers a model yet.')).toBeNull();

		await view.rerender({ sections: [], loading: false });
		expect(screen.getByText('No connected provider offers a model yet.')).toBeTruthy();
	});

	it('shows the sentence the caller passed when nothing can be offered', () => {
		renderDialog({ sections: [], emptyText: 'No connected provider offers a vision model yet.' });

		expect(screen.getByText('No connected provider offers a vision model yet.')).toBeTruthy();
	});

	it('clears the search on every open, so a stale query cannot hide the list', async () => {
		const { view } = renderDialog();

		await fireEvent.input(screen.getByPlaceholderText('Search models'), {
			target: { value: 'o3' }
		});
		expect(screen.queryByRole('button', { name: 'GPT-4o' })).toBeNull();

		await view.rerender({ open: false });
		await view.rerender({ open: true });

		expect(screen.getByRole('button', { name: 'GPT-4o' })).toBeTruthy();
	});

	it('closes from its own button', async () => {
		const { onclose } = renderDialog();

		await fireEvent.click(screen.getByRole('button', { name: 'Done' }));

		expect(onclose).toHaveBeenCalled();
	});
});
