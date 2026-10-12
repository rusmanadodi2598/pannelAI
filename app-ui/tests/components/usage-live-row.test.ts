// Live row tests (src/lib/components/UsageLivePanel.svelte and UsageLiveTabs.svelte).
//
// The live controls row holds only the connection: the status chip, the pause control, and the retry when
// the stream is unavailable. The live facts (requests in flight, the last finished provider, the last
// error) and the finished requests are not in that row. They sit together in one tablist, Live activity,
// at the bottom of the panel, below the drawing, so the page reads Node first and the detail after it.
// Each tab scrolls in its own box.
//
// The tablist is a statement about the stream, not a control over it. The tabs are real tabs so a reader
// can move between the lists, and the facts inside them are never focusable.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { frameText } from '../support/live-stream';
import { frame, startedNow, stubPanel } from '../support/live-panel-stub';

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

/** The connection chip is the row's first control, so it names the row for the tests. */
async function liveRow(): Promise<HTMLElement> {
	const chip = await screen.findByText('Connecting');
	return chip.parentElement as HTMLElement;
}

/** The Live activity tablist, found by its accessible name. */
function liveTabs(): HTMLElement {
	return screen.getByRole('tablist', { name: 'Live activity' });
}

/** Opens one tab of the Live activity tablist, the way a reader does: by clicking it. */
async function openTab(name: RegExp): Promise<void> {
	const tab = await screen.findByRole('tab', { name });
	tab.click();
}

/** The panel of the tab that is open now. */
function openPanel(): HTMLElement {
	const selected = screen.getByRole('tab', { selected: true });
	const panelId = selected.getAttribute('aria-controls');
	if (panelId === null) throw new Error('the selected tab names no panel');
	const panel = document.getElementById(panelId);
	if (panel === null) throw new Error('the selected tab panel is not in the document');
	return panel;
}

/** One active entry the staleness guard keeps, so the In flight tab carries it. */
function sendInFlight(
	stub: { streams: { send: (text: string) => void }[] },
	providerId = 'openai'
) {
	stub.streams[0].send(
		frameText(
			frame({ active: [{ provider_id: providerId, model: 'gpt-4o', started_at: startedNow() }] })
		)
	);
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
});

describe('UsageLivePanel Live activity tab', () => {
	it('sits at the bottom of the panel, after the drawing', async () => {
		stubPanel();
		const { container } = render(UsageLivePanel);
		await liveRow();

		const figure = container.querySelector('figure');
		expect(figure).not.toBeNull();
		// DOCUMENT_POSITION_FOLLOWING: the tablist comes after the drawing in the page.
		const position = (figure as Node).compareDocumentPosition(liveTabs());
		expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
	});

	it('states the in-flight requests in the In flight tab, with the provider in the status colour', async () => {
		const stub = stubPanel();
		const { container } = render(UsageLivePanel);
		await liveRow();

		sendInFlight(stub);
		await openTab(/In flight \(1\)/);

		const panel = openPanel();
		expect(
			within(panel).getByText('OpenAI (gpt-4o)').classList.contains('text-[var(--color-ok)]')
		).toBe(true);
		// The drawing's own frame carries no sentence: the facts live in the tab, never in the figure.
		expect(container.querySelector('figure')?.textContent).not.toMatch(/in flight/);
	});

	it('states the last finished provider in its tab, plainly', async () => {
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
					]
				})
			)
		);

		await openTab(/Last finished/);
		const panel = openPanel();
		expect(within(panel).getByText('OpenAI').classList.contains('text-[var(--color-ok)]')).toBe(
			false
		);
	});

	it('names a provider the registry does not carry by its id', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await liveRow();

		sendInFlight(stub, 'gone-provider');
		await openTab(/In flight \(1\)/);

		// The in-flight fact names the provider and the model it routes, as the drawing's label does.
		expect(openPanel().textContent?.replace(/\s+/g, ' ').trim()).toBe('gone-provider (gpt-4o)');
	});

	it('lists the three tabs in order inside their own tablist, not beside the controls', async () => {
		stubPanel();
		render(UsageLivePanel);
		const row = await liveRow();

		const tabs = within(liveTabs()).getAllByRole('tab');
		expect(tabs.map((tab) => tab.textContent?.trim())).toEqual([
			'In flight (0)',
			'Last finished',
			'Finished requests (0)'
		]);
		expect(liveTabs().parentElement).not.toBe(row);
	});

	it('gives every tab a scroll box of one height, so switching does not move the page', async () => {
		stubPanel();
		render(UsageLivePanel);
		await liveRow();

		for (const name of [/In flight/, /Last finished/, /Finished requests/]) {
			await openTab(name);
			const box = openPanel().firstElementChild as HTMLElement;
			expect(box.classList.contains('max-h-48')).toBe(true);
			expect(box.classList.contains('overflow-y-auto')).toBe(true);
		}
	});

	it('states the empty tabs plainly rather than leaving them blank', async () => {
		stubPanel();
		render(UsageLivePanel);
		await liveRow();

		await openTab(/In flight/);
		expect(openPanel().textContent).toContain('No request is in flight.');
		await openTab(/Last finished/);
		expect(openPanel().textContent).toContain('No request has finished yet.');
	});

	it('keeps the facts out of the figure and the controls row', async () => {
		const stub = stubPanel();
		const { container } = render(UsageLivePanel);
		const row = await liveRow();

		sendInFlight(stub);
		await openTab(/In flight \(1\)/);

		expect(row.textContent).not.toMatch(/in flight/);
		expect(container.querySelector('figure')?.textContent).not.toMatch(/in flight/);
	});
});
