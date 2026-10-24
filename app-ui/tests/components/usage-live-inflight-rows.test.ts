// The requests in flight (src/lib/components/UsageLiveTabs.svelte, the In flight tab).
//
// The tab lists the live requests one row per request, in the drawing's own colour rule, and states how long
// the gateway has been holding each. Two things a reader of this file should not have to work out from the
// code: the row is the accessible statement of a fact the drawing encodes as a lit node, and the box follows
// a request that just arrived rather than the history it never touches.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { squashed } from '../support/dom';
import { liveRow, openPanel, openTab, sendInFlight } from '../support/live-activity';
import { frameText } from '../support/live-stream';
import { frame, startedNow, stubPanel } from '../support/live-panel-stub';
import { provider } from '../support/providers-route-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the In flight rows', () => {
	it('states each request with its provider in the status colour', async () => {
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		sendInFlight(stub);
		await openTab(/In flight \(1\)/);

		expect(
			within(openPanel()).getByText('OpenAI (gpt-4o)').classList.contains('text-[var(--color-ok)]')
		).toBe(true);
	});

	it('names a provider the registry does not carry by its id', async () => {
		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		sendInFlight(stub, 'gone-provider');
		await openTab(/In flight \(1\)/);

		// The row names the provider and the model it routes, as the drawing's label does.
		const panel = openPanel();
		expect(within(panel).getByText('gone-provider (gpt-4o)')).toBeTruthy();
		// It also states how long the gateway has been holding it, which is the reason the tab is open.
		expect(squashed(within(panel).getByText(/^\d+s$/))).toMatch(/^\d+s$/);
	});

	it('lists one row per in-flight request, and states the combo it entered through', async () => {
		const stub = stubPanel({
			providers: [
				provider({ id: 'openai', name: 'OpenAI', endpoint_count: 2 }),
				provider({ id: 'anthropic', name: 'Anthropic', endpoint_count: 1 })
			]
		});
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		stub.streams[0].send(
			frameText(
				frame({
					active: [
						{ provider_id: 'openai', model: 'gpt-4o', started_at: startedNow() },
						{
							provider_id: 'anthropic',
							model: 'claude',
							combo: 'pro-tier',
							started_at: startedNow()
						}
					]
				})
			)
		);
		await openTab(/In flight \(2\)/);

		const rows = within(openPanel()).getAllByRole('listitem');
		expect(rows).toHaveLength(2);
		// Each row names the request it holds: the combo it entered through, then who is answering it.
		expect(squashed(rows[0])).toContain('OpenAI (gpt-4o)');
		expect(squashed(rows[1])).toContain('pro-tier →');
		expect(squashed(rows[1])).toContain('Anthropic (claude)');
	});

	it('follows a request that arrives while the in-flight tab is open', async () => {
		const scrolled = vi.fn();
		HTMLElement.prototype.scrollIntoView = scrolled;

		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		sendInFlight(stub);
		await openTab(/In flight \(1\)/);
		expect(scrolled, 'opening a tab must not scroll').not.toHaveBeenCalled();

		stub.streams[0].send(
			frameText(
				frame({
					active: [
						{ provider_id: 'anthropic', model: 'claude', started_at: startedNow() },
						{ provider_id: 'openai', model: 'gpt-4o', started_at: startedNow() }
					]
				})
			)
		);
		await screen.findByRole('tab', { name: 'In flight (2)' });

		expect(scrolled).toHaveBeenCalledTimes(1);
		expect(scrolled).toHaveBeenCalledWith({ block: 'nearest' });
	});

	// The finished list is newest-first and read like a history, so the box leaves whatever position the
	// reader took on purpose.
	it('does not follow the finished list', async () => {
		const scrolled = vi.fn();
		HTMLElement.prototype.scrollIntoView = scrolled;

		const stub = stubPanel();
		render(UsageLivePanel, { props: stub.props });
		await liveRow();

		await openTab(/Finished requests/);
		for (const id of ['req_1', 'req_2']) {
			stub.streams[0].send(
				frameText(
					frame({
						recent: [
							{ request_id: id, provider_id: 'openai', ts: '2026-09-27T09:00:00Z' },
							...(id === 'req_2'
								? [{ request_id: 'req_1', provider_id: 'openai', ts: '2026-09-27T09:00:00Z' }]
								: [])
						]
					})
				)
			);
		}
		await screen.findByRole('tab', { name: 'Finished requests (2)' });

		expect(scrolled).not.toHaveBeenCalled();
	});
});
