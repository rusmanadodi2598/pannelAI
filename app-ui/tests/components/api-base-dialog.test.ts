// API base dialog tests (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// The dialog is the panel's one place that hands out the gateway address in more than one form, so what is
// worth asserting is that each tab copies exactly the value it shows, and that a refused clipboard write is
// visible. The refusal is the case that used to pass unnoticed: the header's old control kept its failure
// sentence inside an `sr-only` span, so a refused write read as a dead button.
//
// The address is read from the panel server rather than derived from the browser, and a live click-through
// is why: the dialog advertised `location.origin`, which on the `http://<host>:3000` address the panel
// normally runs at is a bind-all address no client can call. So the read's outcomes are all driven here,
// and every failure row also asserts that the browser's own origin is nowhere on screen.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ApiBaseDialog from '../../src/lib/components/ApiBaseDialog.svelte';
import { BASE_URL, stubApiBase } from '../support/api-base-stub';
import { expectIconOnly } from '../support/icon-only';

const LOADING = 'Reading the gateway address from the panel server.';
const FAILED_COPY = 'Copy failed. Select the text and copy it.';
const RETRY = 'Try again';

// Each tab's value is composed from the address the stub serves rather than from the environment, so a
// dialog that derived the address from the browser fails this table.
const TAB_CASES = [
	{ name: 'the base URL tab', tab: 'Base URL', expected: BASE_URL },
	{
		name: 'the cURL tab',
		tab: 'cURL',
		expected: `curl -H "Authorization: Bearer sk-..." ${BASE_URL}/models`
	},
	{
		name: 'the OpenAI client tab',
		tab: 'OpenAI client',
		expected: `OPENAI_BASE_URL=${BASE_URL}\nOPENAI_API_KEY=sk-...`
	}
];

function stubClipboard(writeText: (value: string) => Promise<void>): void {
	Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
}

function open(): void {
	render(ApiBaseDialog, { props: { open: true, onclose: () => {} } });
}

/** Opens the dialog against a served address and waits for the answer to arrive. */
async function openLoaded(): Promise<void> {
	stubApiBase();
	open();
	await screen.findByRole('tab', { name: 'Base URL' });
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	delete (navigator as { clipboard?: unknown }).clipboard;
	delete (document as { execCommand?: unknown }).execCommand;
});

describe('api base dialog', () => {
	it('says it is reading until the panel answers, and shows the tabs with the answer', async () => {
		const stub = stubApiBase({ hold: true });
		open();

		// "Nothing read yet" is a state of its own: the address cannot be guessed while the read is in
		// flight, which is exactly the guess the old dialog made.
		expect(screen.getByText(LOADING)).toBeTruthy();
		expect(screen.queryByRole('tab')).toBeNull();

		stub.release();

		expect(await screen.findByRole('tab', { name: 'Base URL' })).toBeTruthy();
		expect(screen.queryByText(LOADING)).toBeNull();
	});

	it('copies the value each tab shows, and says so', async () => {
		const copied: string[] = [];
		stubClipboard(async (value) => {
			copied.push(value);
		});
		await openLoaded();

		for (const testCase of TAB_CASES) {
			await fireEvent.click(screen.getByRole('tab', { name: testCase.tab }));
			const panel = screen.getByRole('tabpanel');

			// The block and the clipboard carry the same string, which is the one invariant every tab
			// shares: a copy control that copied something else would be a credential hazard, not a bug.
			const shown = panel.querySelector('pre')?.textContent ?? '';
			expect(shown.trim()).toBe(testCase.expected);

			await fireEvent.click(within(panel).getByRole('button', { name: 'Copy' }));

			// The write is asynchronous on both paths, so the outcome is waited for rather than read off
			// the tick the click returned on.
			expect(await within(panel).findByText('Copied.')).toBeTruthy();
			expect(copied.at(-1)).toBe(testCase.expected);
		}
	});

	it('offers the copy control as an icon-only button on every tab', async () => {
		await openLoaded();

		for (const testCase of TAB_CASES) {
			await fireEvent.click(screen.getByRole('tab', { name: testCase.tab }));

			const panel = screen.getByRole('tabpanel');
			expectIconOnly(within(panel).getByRole('button', { name: 'Copy' }), 'Copy');
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
			await openLoaded();

			await fireEvent.click(screen.getByRole('button', { name: 'Copy' }));

			// The sentence has to be readable, not announced only: the value stays on screen beside it, and
			// a failure nobody sees is the defect this control was rewritten to remove.
			const status = await screen.findByText(FAILED_COPY);
			expect(status.closest('.sr-only')).toBeNull();
			expect(screen.queryByText('Copied.')).toBeNull();

			cleanup();
		}
	});

	it('reports a failed read, never falls back to the panel origin, and reads again on request', async () => {
		const cases = [
			{
				name: 'the panel refuses the read',
				options: { status: 503, message: 'The gateway address is unavailable.' },
				expected: 'The gateway address is unavailable.'
			},
			{
				name: 'the answer is not the shape the panel reads',
				options: { body: { base: BASE_URL } },
				expected: 'base_url'
			}
		];

		for (const testCase of cases) {
			const stub = stubApiBase(testCase.options);
			open();

			const alert = await screen.findByRole('alert');
			expect(alert.textContent).toContain(testCase.expected);

			// The defect this replaces: a failed read still showed a URL that looked right, because the
			// dialog derived it from the browser. The panel's own origin must not appear anywhere, which is
			// why the stub serves an address on another host.
			expect(document.body.textContent).not.toContain(location.origin);
			expect(screen.queryByRole('tab')).toBeNull();

			// Both fields move, because a retry has to be answered rather than repeated: the drift case
			// fails on its body, so a status alone would leave the second read just as unreadable.
			stub.status = 200;
			stub.body = undefined;
			await fireEvent.click(screen.getByRole('button', { name: RETRY }));

			expect(await screen.findByRole('tab', { name: 'Base URL' })).toBeTruthy();
			expect(screen.queryByRole('alert')).toBeNull();
			expect(stub.reads).toHaveLength(2);

			cleanup();
		}
	});

	it('reads the address again on every open, rather than describing the previous panel', async () => {
		const stub = stubApiBase();
		const onclose = () => {};
		const view = render(ApiBaseDialog, { props: { open: true, onclose } });
		await screen.findByRole('tab', { name: 'Base URL' });

		await view.rerender({ open: false, onclose });
		await view.rerender({ open: true, onclose });

		// The address comes from the panel's environment, so a panel restarted against another gateway must
		// not be described by the answer the previous open happened to get.
		expect(await screen.findByRole('tab', { name: 'Base URL' })).toBeTruthy();
		expect(stub.reads).toHaveLength(2);
	});
});
