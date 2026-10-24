// The Live activity tablist (src/lib/components/UsageLiveTabs.svelte).
//
// What the tablist *is*: where it sits in the panel, which tabs it lists, the box each one scrolls in, and
// the statements it makes when a tab holds nothing. The rows a tab holds are the subject of
// `usage-live-inflight-rows.test.ts`, which is the half that fails when a frame is misread.
//
// The tablist is a statement about the stream, not a control over it. The tabs are real tabs so a reader can
// move between the lists, and nothing inside a panel is focusable.

import { cleanup, render, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { liveRow, liveTabs, openPanel, openTab, sendInFlight } from '../support/live-activity';
import { frameText } from '../support/live-stream';
import { frame, stubPanel } from '../support/live-panel-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the Live activity tablist', () => {
	it('sits at the bottom of the panel, after the drawing', async () => {
		const stub = stubPanel();
		const { container } = render(UsageLivePanel, { props: stub.props });
		await liveRow();

		const figure = container.querySelector('figure');
		expect(figure).not.toBeNull();
		// DOCUMENT_POSITION_FOLLOWING: the tablist comes after the drawing in the page.
		const position = (figure as Node).compareDocumentPosition(liveTabs());
		expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
	});

	it('frames the region, so the three tabs read as one box at the bottom of the page', async () => {
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		// PanelTabs draws the tablist inside its own column, so the region is one level further out.
		const region = liveTabs().parentElement?.parentElement as HTMLElement;
		expect(region.classList.contains('border')).toBe(true);
		expect(region.classList.contains('border-[var(--color-border)]')).toBe(true);
		expect(region.classList.contains('rounded-[var(--radius-md)]')).toBe(true);
		// The same frame holds the tab strip and the list it selects, which is what makes it one component.
		expect(region.contains(openPanel())).toBe(true);
	});

	it('scrolls the strip rather than clipping a tab when the labels outrun a phone', async () => {
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		expect(liveTabs().classList.contains('overflow-x-auto')).toBe(true);
		for (const tab of within(liveTabs()).getAllByRole('tab')) {
			expect(tab.classList.contains('shrink-0')).toBe(true);
		}
	});

	it('lists the three tabs in order inside their own tablist, not beside the controls', async () => {
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
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
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		const boxes: string[] = [];
		for (const name of [/In flight/, /Last finished/, /Finished requests/]) {
			await openTab(name);
			const box = openPanel().firstElementChild as HTMLElement;
			expect(box.classList.contains('h-48')).toBe(true);
			expect(box.classList.contains('overflow-y-auto')).toBe(true);
			boxes.push(box.className);
		}
		// One box, three tabs: any difference here is a tab that resizes the region when it opens.
		expect(new Set(boxes).size).toBe(1);
	});

	it('states the empty tabs plainly rather than leaving them blank', async () => {
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		await openTab(/In flight/);
		expect(openPanel().textContent).toContain('No request is in flight.');
		await openTab(/Last finished/);
		expect(openPanel().textContent).toContain('No request has finished yet.');
	});

	it('states the last finished provider in its tab, plainly', async () => {
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
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
		// The Last finished tab carries no count, so its label is the same before and after the frame and
		// opening it does not wait for the stream. The row is awaited, not read on the next tick.
		await within(panel).findByText('OpenAI');
		expect(within(panel).getByText('OpenAI').classList.contains('text-[var(--color-ok)]')).toBe(
			false
		);
	});

	it('keeps the facts out of the figure and the controls row', async () => {
		const stub = stubPanel();
		const { container } = render(UsageLivePanel, { props: stub.props });
		const row = await liveRow();

		sendInFlight(stub);
		await openTab(/In flight \(1\)/);

		expect(row.textContent).not.toMatch(/in flight/);
		expect(container.querySelector('figure')?.textContent).not.toMatch(/in flight/);
	});
});
