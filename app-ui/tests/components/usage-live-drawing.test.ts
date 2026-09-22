// Live panel drawing tests (src/lib/components/UsageLivePanel.svelte, draft 012 F2 and F3).
//
// The panel reads twice, and these rows are about what those reads put on the screen: the node set from
// the registry, the state each node is in from the stream, and the list of requests the frame says have
// finished. The rows about a failed or shortened registry read are here because the notice for one is the
// panel's, not the drawing's.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { frameText } from '../support/live-stream';
import { frame, startedNow, stubPanel } from '../support/live-panel-stub';
import { provider } from '../support/providers-route-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('UsageLivePanel drawing', () => {
	it('draws one node per provider that has an endpoint or needs no credential', async () => {
		stubPanel({
			providers: [
				provider({ id: 'openai', name: 'OpenAI', endpoint_count: 2 }),
				provider({ id: 'anthropic', name: 'Anthropic', endpoint_count: 0 }),
				provider({ id: 'opencode', name: 'OpenCode', no_auth: true })
			]
		});
		render(UsageLivePanel);

		expect(await screen.findByText('2 providers are configured: OpenAI, OpenCode.')).toBeTruthy();
		expect(screen.queryByText('Anthropic')).toBeNull();
	});

	it('pulses the node of the provider the frame says is routing', async () => {
		const stub = stubPanel();
		const container = render(UsageLivePanel).container;
		await screen.findByText('Connecting');

		stub.streams[0].send(
			frameText(
				frame({ active: [{ provider_id: 'openai', model: 'gpt-4o', started_at: startedNow() }] })
			)
		);

		expect(await screen.findByText(/1 in flight: OpenAI \(gpt-4o\)\./)).toBeTruthy();
		expect(container.querySelector('.animate-ping')).toBeTruthy();
	});

	it('stops the drawing when the operator pauses, and keeps the state it last saw', async () => {
		const stub = stubPanel();
		const container = render(UsageLivePanel).container;
		await screen.findByText('Connecting');

		stub.streams[0].send(
			frameText(
				frame({ active: [{ provider_id: 'openai', model: 'gpt-4o', started_at: startedNow() }] })
			)
		);
		// Awaited on the drawing's own sentence rather than on the status chip: the drawing only exists
		// once the registry read lands, and a row about motion must not pass on an empty drawing.
		expect(await screen.findByText(/1 in flight: OpenAI \(gpt-4o\)\./)).toBeTruthy();
		expect(container.querySelector('.animate-flow')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Pause live updates' }));

		expect(await screen.findByText('Paused')).toBeTruthy();
		expect(container.querySelector('.animate-flow')).toBeNull();
		expect(container.querySelector('.animate-ping')).toBeNull();
		expect(
			(screen.getByText('OpenAI').parentElement as HTMLElement).classList.contains(
				'border-[var(--color-ok)]'
			)
		).toBe(true);
	});

	it('states that the registry could not be read, so the drawing is empty', async () => {
		stubPanel({ providersStatus: 500, providersMessage: 'the registry is unreachable' });
		render(UsageLivePanel);

		expect(
			await screen.findByText(
				/Providers could not be read \(the registry is unreachable\), so the drawing has no nodes\./
			)
		).toBeTruthy();
		expect(screen.getByText('No provider is configured')).toBeTruthy();
	});

	it('states that the registry is longer than the drawing reads', async () => {
		stubPanel({ total: 120 });
		render(UsageLivePanel);

		expect(
			await screen.findByText(/carries 120 providers and this drawing reads the first 1\./)
		).toBeTruthy();
	});

	it('lists the requests the frame says have finished', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await screen.findByText('Connecting');

		stub.streams[0].send(
			frameText(
				frame({
					recent: [
						{
							request_id: 'req_1',
							provider_id: 'openai',
							model: 'gpt-4o',
							ts: '2026-09-22T09:00:00Z',
							status: 'success'
						},
						{
							request_id: 'req_2',
							provider_id: 'anthropic',
							model: 'claude-3-5-sonnet',
							ts: '2026-09-22T09:01:00Z',
							status: 'error'
						}
					]
				})
			)
		);

		expect(await screen.findByText('Finished requests')).toBeTruthy();
		expect(screen.getByText('gpt-4o')).toBeTruthy();
		expect(screen.getByText('claude-3-5-sonnet')).toBeTruthy();
		expect(screen.getByText('Success')).toBeTruthy();
		expect(screen.getByText('Error')).toBeTruthy();
	});

	it('does not render the finished list when the frame reports none', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await screen.findByText('Connecting');

		stub.streams[0].send(frameText(frame()));

		expect(await screen.findByText('Live')).toBeTruthy();
		expect(screen.queryByText('Finished requests')).toBeNull();
		// Awaited, because the drawing's own sentence waits on the registry read rather than on the frame.
		expect(
			await screen.findByText(/No request has finished since this screen opened\./)
		).toBeTruthy();
	});
});
