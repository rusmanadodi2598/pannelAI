// API base dialog tests (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// The dialog is the panel's one place that hands out the gateway address in more than one form, so what
// is worth asserting is that each tab copies exactly the value it shows, and that a refused clipboard
// write is visible. The refusal is the case that used to pass unnoticed: the header's old control kept
// its failure sentence inside an `sr-only` span, so a refused write read as a dead button.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import ApiBaseDialog from '../../src/lib/components/ApiBaseDialog.svelte';

// The address is the environment's own origin, so the expected strings are derived here rather than
// written out: a dialog that hardcoded a host would fail this table.
const BASE = `${location.origin}/api/v1`;

const TAB_CASES = [
	{ name: 'the base URL tab', tab: 'Base URL', expected: BASE },
	{
		name: 'the cURL tab',
		tab: 'cURL',
		expected: `curl -H "Authorization: Bearer sk-..." ${BASE}/models`
	},
	{
		name: 'the OpenAI client tab',
		tab: 'OpenAI client',
		expected: `OPENAI_BASE_URL=${BASE}\nOPENAI_API_KEY=sk-...`
	}
];

const FAILED = 'Copy failed. Select the text and copy it.';

function stubClipboard(writeText: (value: string) => Promise<void>): void {
	Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
}

function open(): void {
	render(ApiBaseDialog, { props: { open: true, onclose: () => {} } });
}

afterEach(() => {
	cleanup();
	delete (navigator as { clipboard?: unknown }).clipboard;
});

describe('api base dialog', () => {
	it('copies the value each tab shows, and says so', async () => {
		const copied: string[] = [];
		stubClipboard(async (value) => {
			copied.push(value);
		});
		open();

		for (const testCase of TAB_CASES) {
			await fireEvent.click(screen.getByRole('tab', { name: testCase.tab }));
			const panel = screen.getByRole('tabpanel');

			// The block and the clipboard carry the same string, which is the one invariant every tab
			// shares: a copy control that copied something else would be a credential hazard, not a bug.
			const shown = panel.querySelector('pre')?.textContent ?? '';
			expect(shown.trim()).toBe(testCase.expected);

			await fireEvent.click(within(panel).getByRole('button', { name: 'Copy' }));

			expect(copied.at(-1)).toBe(testCase.expected);
			expect(within(panel).getByText('Copied.')).toBeTruthy();
		}
	});

	it('reports a refused write where the operator can see it', async () => {
		const cases = [
			{
				name: 'the browser refuses the write',
				refuse: () => stubClipboard(async () => Promise.reject(new Error('denied')))
			},
			{
				name: 'the clipboard API is absent, as it is outside a secure context',
				refuse: () => delete (navigator as { clipboard?: unknown }).clipboard
			}
		];

		for (const testCase of cases) {
			testCase.refuse();
			open();

			await fireEvent.click(screen.getByRole('button', { name: 'Copy' }));

			// The sentence has to be readable, not announced only: the value stays on screen beside it,
			// and a failure nobody sees is the defect this control was rewritten to remove.
			const status = screen.getByText(FAILED);
			expect(status.closest('.sr-only')).toBeNull();
			expect(screen.queryByText('Copied.')).toBeNull();

			cleanup();
		}
	});
});
