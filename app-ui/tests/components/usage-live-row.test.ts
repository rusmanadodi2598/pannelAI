// The live controls row (src/lib/components/UsageLivePanel.svelte).
//
// The row holds only the connection: the status chip, the pause control, and the retry when the stream is
// unavailable. What the stream states is not here: that is the Live activity tablist at the bottom of the
// panel, asserted in `usage-live-activity-tabs.test.ts`. The two split on why they fail, since the row fails
// on a control's box and the tablist fails on a fact's wording.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { liveRow } from '../support/live-activity';
import { stubPanel } from '../support/live-panel-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

// The one box every control on the row wears: 44px minimum height (SPEC-UI §8.6 rule 3, the panel's floor
// for a control's touch target), the panel's chip radius, one border, and the same horizontal padding.
const CONTROL_BOX = [
	'inline-flex',
	'min-h-11',
	'items-center',
	'rounded-[var(--radius-sm)]',
	'border',
	'border-[var(--color-border)]',
	'px-3',
	'text-sm'
];

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the live controls row', () => {
	it('puts the status chip and the pause control on one wrapping row', async () => {
		stubPanel();
		render(UsageLivePanel);
		const row = await liveRow();

		expect(row.classList.contains('flex')).toBe(true);
		expect(row.classList.contains('flex-wrap')).toBe(true);
		expect(row.classList.contains('items-stretch')).toBe(true);
		expect(screen.getByRole('button', { name: 'Pause live updates' }).parentElement).toBe(row);
	});

	it('gives the chip and the pause control the same box', async () => {
		stubPanel();
		render(UsageLivePanel);
		const chip = await screen.findByText('Connecting');
		const pause = screen.getByRole('button', { name: 'Pause live updates' });

		for (const element of [chip, pause]) {
			for (const token of CONTROL_BOX) {
				expect(element.classList.contains(token)).toBe(true);
			}
		}
	});

	it('gives the retry control the same box when it appears', async () => {
		stubPanel({ liveAnswers: [() => new Response('', { status: 404 })] });
		render(UsageLivePanel);
		const row = await liveRow();

		const retry = await screen.findByRole('button', { name: 'Try again' });
		expect(retry.parentElement).toBe(row);
		for (const token of CONTROL_BOX) {
			expect(retry.classList.contains(token)).toBe(true);
		}
	});

	it('marks only the controls as hoverable, never the statements', async () => {
		stubPanel({ liveAnswers: [() => new Response('', { status: 404 })] });
		render(UsageLivePanel);
		await liveRow();

		const retry = await screen.findByRole('button', { name: 'Try again' });
		// The answer a pointer gets is the state scale's, not a surface the row picked for itself.
		for (const control of [retry, screen.getByRole('button', { name: 'Pause live updates' })]) {
			expect(control.classList.contains('hover:bg-[var(--color-state-hover)]')).toBe(true);
			expect(control.classList.contains('active:bg-[var(--color-state-press)]')).toBe(true);
		}
		expect(
			screen.getByText('Unavailable').classList.contains('hover:bg-[var(--color-state-hover)]')
		).toBe(false);
	});
});
