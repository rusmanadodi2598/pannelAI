// The combo editor's strategy-driven fields (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The rule asserted here is that the field a strategy ignores is hidden, not disabled, so the panel never
// shows a control that cannot affect the save. What the form sends, and what a refused save leaves on
// screen, is `combo-editor-save.test.ts`; the model list and its picker are `combo-editor-models.test.ts`.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ComboEditor from '../../src/lib/components/ComboEditor.svelte';
import { selectStrategy, stubFetch } from '../support/combo-editor-harness';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('ComboEditor strategy fields', () => {
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
		expect(screen.getByText('Spread across models, keeping requests sticky on one.')).toBeTruthy();
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
});
