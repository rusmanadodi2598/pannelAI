// Live row tests (src/lib/components/UsageLivePanel.svelte, draft 035 F1 and F2).
//
// The owner's correction of 2026-09-27 asked for two things on one row: every tab in the live controls
// carries the same box (F1), and the dynamic facts sentence left the drawing's frame, where it moved the
// node animation, to become a tab of that row (F2). The rows here pin the shape, not the pixels: what the
// browser measures belongs to the live click-through (draft 035 §7.2), and what the DOM promises is here.
// The tab is a statement, not a control (draft 023 F2's rule), which is why nothing in these rows can be
// clicked or focused, and why only the two real controls carry the hover affordance.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { frameText } from '../support/live-stream';
import { frame, startedNow, stubPanel } from '../support/live-panel-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

// The one box every tab on the row wears: 44px minimum height (SPEC-UI §8.6 rule 3, the panel's floor
// for a control's touch target), the panel's chip radius, one border, and the same horizontal padding.
// The row's own two controls already had it; F1 lifted the chip to join them.
const TAB_BOX = [
	'inline-flex',
	'min-h-11',
	'items-center',
	'rounded-[var(--radius-sm)]',
	'border',
	'border-[var(--color-border)]',
	'px-3',
	'text-sm'
];

function rowOf(chip: HTMLElement): HTMLElement {
	return chip.parentElement as HTMLElement;
}

/** The connection chip is the row's first tab, so it names the row for the tests. */
async function liveRow(): Promise<HTMLElement> {
	const chip = await screen.findByText('Connecting');
	return rowOf(chip);
}

/** One active entry the staleness guard keeps, so the facts tab renders and is awaited. */
async function withInFlight(stub: { streams: { send: (text: string) => void }[] }): Promise<void> {
	stub.streams[0].send(
		frameText(
			frame({ active: [{ provider_id: 'openai', model: 'gpt-4o', started_at: startedNow() }] })
		)
	);
	await screen.findByText('1 in flight:');
}

function tabFor(label: HTMLElement | string): HTMLElement {
	const found =
		typeof label === 'string' ? screen.getByText(label).parentElement : label.parentElement;
	if (found === null) {
		throw new Error('the fact label rendered without a parent tab');
	}
	return found;
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('UsageLivePanel row', () => {
	it('puts the status chip and the pause control on one wrapping row', async () => {
		stubPanel();
		render(UsageLivePanel);
		const row = await liveRow();

		expect(row.classList.contains('flex')).toBe(true);
		expect(row.classList.contains('flex-wrap')).toBe(true);
		expect(row.classList.contains('items-stretch')).toBe(true);
		expect(rowOf(screen.getByRole('button', { name: 'Pause live updates' }))).toBe(row);
	});

	it('gives every tab it renders, facts tab included, the same box', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		const chip = await screen.findByText('Connecting');
		const row = rowOf(chip);
		await withInFlight(stub);

		const tab = tabFor('1 in flight:');
		const pause = screen.getByRole('button', { name: 'Pause live updates' });
		expect(tab.parentElement).toBe(row);

		for (const element of [chip, tab, pause]) {
			for (const token of TAB_BOX) {
				expect(element.classList.contains(token)).toBe(true);
			}
		}
	});

	it('lets the facts tab shrink and wrap instead of widening the page', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await liveRow();
		await withInFlight(stub);

		const tab = tabFor('1 in flight:');
		expect(tab.classList.contains('min-w-0')).toBe(true);
		expect(tab.classList.contains('flex-wrap')).toBe(true);
		expect(within(tab).getByText('OpenAI (gpt-4o)').classList.contains('break-words')).toBe(true);
	});

	it('gives the retry tab the same box when it appears', async () => {
		stubPanel({ liveAnswers: [() => new Response('', { status: 404 })] });
		render(UsageLivePanel);
		const row = await liveRow();

		const retry = await screen.findByRole('button', { name: 'Try again' });
		expect(retry.parentElement).toBe(row);
		for (const token of TAB_BOX) {
			expect(retry.classList.contains(token)).toBe(true);
		}
	});

	it('marks only the two controls as hoverable, never the statements', async () => {
		stubPanel({ liveAnswers: [() => new Response('', { status: 404 })] });
		render(UsageLivePanel);
		await liveRow();

		const retry = await screen.findByRole('button', { name: 'Try again' });
		expect(retry.classList.contains('hover:bg-[var(--color-surface-2)]')).toBe(true);
		expect(
			screen
				.getByRole('button', { name: 'Pause live updates' })
				.classList.contains('hover:bg-[var(--color-surface-2)]')
		).toBe(true);
		expect(
			screen.getByText('Unavailable').classList.contains('hover:bg-[var(--color-surface-2)]')
		).toBe(false);
	});

	it('states the live facts in a tab of the row, not in the drawing frame', async () => {
		const stub = stubPanel();
		const container = render(UsageLivePanel).container;
		const row = await liveRow();

		stub.streams[0].send(
			frameText(
				frame({
					active: [{ provider_id: 'openai', model: 'gpt-4o', started_at: startedNow() }],
					error_provider: 'openai'
				})
			)
		);

		const tab = tabFor(await within(row).findByText('1 in flight:'));
		expect(tab.parentElement).toBe(row);
		// The sentence keeps its spaces in text content even though label and value are separate spans.
		expect((tab.textContent ?? '').replace(/\s+/g, ' ')).toContain('1 in flight: OpenAI (gpt-4o)');
		expect(
			within(tab).getByText('OpenAI (gpt-4o)').classList.contains('text-[var(--color-ok)]')
		).toBe(true);
		// The drawing's own frame carries no sentence: the paragraph that moved the beam is gone (F2).
		expect(container.querySelector('figure')?.textContent).not.toMatch(/in flight|Last error/);
	});

	it('states the finished fact plainly and separates the facts without a sentence period', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await liveRow();

		stub.streams[0].send(
			frameText(
				frame({
					recent: [
						{
							request_id: 'req_1',
							provider_id: 'openai',
							model: 'gpt-4o',
							ts: '2026-09-27T09:00:00Z',
							status: 'success'
						}
					],
					error_provider: 'anthropic'
				})
			)
		);

		const tab = tabFor(await screen.findByText('Last finished:'));
		expect(within(tab).getByText('OpenAI').classList.contains('text-[var(--color-ok)]')).toBe(
			false
		);
		expect(screen.getByText('Last error:')).toBeTruthy();
		expect(tab.textContent).toContain('·');
	});

	it('names a provider the registry does not carry by its id', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await liveRow();

		stub.streams[0].send(
			frameText(frame({ active: [{ provider_id: 'gone-provider', started_at: startedNow() }] }))
		);

		const tab = tabFor(await screen.findByText('1 in flight:'));
		expect((tab.textContent ?? '').replace(/\s+/g, ' ').trim()).toBe('1 in flight: gone-provider');
	});

	it('renders the facts tab as a statement no reader can click or focus', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await liveRow();
		await withInFlight(stub);

		const tab = tabFor('1 in flight:');
		expect(tab.closest('button')).toBeNull();
		expect(tab.tagName).toBe('SPAN');
		expect(tab.hasAttribute('tabindex')).toBe(false);
		expect(screen.queryByRole('tab')).toBeNull();
	});

	it('renders no facts tab while the stream has reported nothing that happened', async () => {
		stubPanel();
		render(UsageLivePanel);
		const row = await liveRow();

		expect(screen.queryByText(/in flight:/)).toBeNull();
		expect(screen.queryByText(/Last finished:/)).toBeNull();
		expect(screen.queryByText(/Last error:/)).toBeNull();
		// The tab count is the positive premise of the silence: chip and pause, nothing else.
		expect(row.children.length).toBe(2);
	});
});
