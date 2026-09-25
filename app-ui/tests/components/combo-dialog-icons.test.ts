// The Combo & Vision dialogs' footer controls (owner directive, 2026-09-25; SPEC-UI §8.11).
//
// Split from `combo-vision-icons.test.ts` by concern: that file holds the row actions and the toolbar, this
// one holds the modal footers and the picker, which share the glyph + label shape PORT 002 gave the
// provider screens' controls. The last case pins a size the click-through measured and the design
// direction settles: the dialog close control is 36px by `DESIGN.md` §7, not the 44px a first reading
// assumed, and a silent change would override recorded owner direction.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ComboDeleteDialog from '../../src/lib/components/ComboDeleteDialog.svelte';
import ComboStrategyFields from '../../src/lib/components/ComboStrategyFields.svelte';
import ComboTestDialog from '../../src/lib/components/ComboTestDialog.svelte';
import ModelPickerDialog from '../../src/lib/components/ModelPickerDialog.svelte';
import VisionModelPicker from '../../src/lib/components/VisionModelPicker.svelte';
import { schemaCombo, type Combo } from '$lib/schemas/combo';
import type { PickerSection } from '$lib/schemas/model-picker';

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

describe('the combo dialogs', () => {
	it('renders the delete confirmation footer as glyph + label', () => {
		render(ComboDeleteDialog, {
			props: {
				combo: combo(),
				error: null,
				conflict: false,
				deleting: false,
				onconfirm: () => {},
				oncancel: () => {}
			}
		});

		for (const name of ['Keep it', 'Delete the combo']) {
			const button = screen.getByRole('button', { name });
			expect(button.querySelector('svg'), `${name} has no glyph`).toBeTruthy();
		}
	});

	it('renders the test readout footer as glyph + label', async () => {
		vi.stubGlobal('fetch', async () => {
			return new Response(JSON.stringify({ results: [] }), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});
		});

		render(ComboTestDialog, { props: { combo: combo(), onclose: () => {} } });

		for (const name of ['Close', 'Test the chain']) {
			const button = screen.getByRole('button', { name });
			expect(button.querySelector('svg'), `${name} has no glyph`).toBeTruthy();
		}
	});
});

describe('the picker dialogs', () => {
	it('renders the picker footer as glyph + label', () => {
		render(ModelPickerDialog, {
			props: {
				title: 'Add models',
				open: true,
				sections,
				selected: [],
				emptyText: 'No connected provider offers a model yet.',
				ontoggle: () => {},
				onclose: () => {}
			}
		});

		expect(
			screen.getByRole('button', { name: 'Done' }).querySelector('svg'),
			'Done has no glyph'
		).toBeTruthy();
	});

	it('renders the vision picker add control as glyph + label', () => {
		render(VisionModelPicker, {
			props: {
				form: { enabled: false, roundRobin: false, models: [] },
				sections,
				pickerFailed: false,
				ontoggle: () => {},
				onchoose: () => {}
			}
		});

		expect(
			screen.getByRole('button', { name: 'Add models' }).querySelector('svg'),
			'Add models has no glyph'
		).toBeTruthy();
	});

	it('gives the dialog close control the 36px size DESIGN.md §7 fixes for a dialog', () => {
		// The live click-through on 2026-09-25 measured this control at 36px and the first reading called it a
		// defect against R-03's 44px. It is not: `DESIGN.md` §7 fixes 36px for a dialog's own controls and
		// reserves 44px for the sidebar, which is a touch surface at every breakpoint. The earlier pass that
		// shipped the icon-only copy control recorded the same decision (`app-ui/README.md`, 2026-09-24). A
		// silent change to 44px would have overridden recorded owner direction, so the size is pinned here
		// instead, and the port document records the reading as deliberate rather than as a fix.
		render(ModelPickerDialog, {
			props: {
				title: 'Add models',
				open: true,
				sections,
				selected: [],
				emptyText: 'No connected provider offers a model yet.',
				ontoggle: () => {},
				onclose: () => {}
			}
		});

		const close = screen.getByRole('button', { name: 'Close dialog' });
		expect(close.className, 'the dialog close control left the 36px DESIGN.md §7 fixes').toContain(
			'size-9'
		);
	});
});

describe('the judge field', () => {
	it('renders Choose as glyph + label, and it opens the picker', async () => {
		const onchoose = vi.fn();
		render(ComboStrategyFields, {
			props: {
				strategy: 'fusion',
				stickyLimit: 1,
				judgeModel: '',
				onchoosejudge: onchoose
			}
		});

		const button = screen.getByRole('button', { name: 'Choose' });
		expect(button.querySelector('svg'), 'Choose has no glyph').toBeTruthy();

		await fireEvent.click(button);
		expect(onchoose).toHaveBeenCalledOnce();
	});
});
