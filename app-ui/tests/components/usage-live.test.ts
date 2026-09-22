// Live panel connection tests (src/lib/components/UsageLivePanel.svelte, draft 012 F2).
//
// The panel owns the stream, so these rows are about what an operator reads and can do about the
// connection: the label, the reason it is not live, and the two controls. What the two reads put on the
// screen is `usage-live-drawing.test.ts`; what the reader does between frames is `usage-live.test.ts` in
// `tests/utils/`.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageLivePanel from '../../src/lib/components/UsageLivePanel.svelte';
import { frameText, liveStreams } from '../support/live-stream';
import { frame, stubPanel } from '../support/live-panel-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('UsageLivePanel connection', () => {
	it('reports live only once a frame has arrived', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);

		expect(await screen.findByText('Connecting')).toBeTruthy();
		expect(screen.queryByText('Live')).toBeNull();

		stub.streams[0].send(frameText(frame()));
		expect(await screen.findByText('Live')).toBeTruthy();
		expect(screen.getByText(/Last frame /)).toBeTruthy();
	});

	it('names the cause when the route is not served', async () => {
		stubPanel({ liveAnswers: [() => new Response('', { status: 404 })] });
		render(UsageLivePanel);

		expect(await screen.findByText('Unavailable')).toBeTruthy();
		expect(screen.getByText(/has no live stream route yet \(it answered 404\)/)).toBeTruthy();
	});

	it('reads again when the operator asks it to, and goes live on the next frame', async () => {
		const { answer, streams } = liveStreams();
		const stub = stubPanel({ liveAnswers: [() => new Response('', { status: 404 }), answer] });
		render(UsageLivePanel);
		await screen.findByText('Unavailable');

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));
		expect(await screen.findByText('Connecting')).toBeTruthy();
		expect(stub.liveCalls()).toBe(2);

		streams[0].send(frameText(frame()));
		expect(await screen.findByText('Live')).toBeTruthy();
	});

	it('stops reading while paused and reads again on resume', async () => {
		const stub = stubPanel();
		render(UsageLivePanel);
		await screen.findByText('Connecting');

		await fireEvent.click(screen.getByRole('button', { name: 'Pause live updates' }));
		expect(screen.getByText('Paused')).toBeTruthy();
		expect(stub.liveInits[0]?.signal?.aborted).toBe(true);

		await fireEvent.click(screen.getByRole('button', { name: 'Resume live updates' }));
		await waitFor(() => expect(stub.liveCalls()).toBe(2));
	});
});
