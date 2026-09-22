// Usage live stop-condition tests (src/lib/usage-live.ts, draft 012 F2).
//
// §8.6.1 requires a self-refreshing screen to be pausable and to stop while the tab is hidden. The stream
// is the third medium that rule applies to, and these rows are what it means for one: an aborted request,
// no scheduled attempt, and a read on the way back. Teardown is here for the same reason, because a
// listener left behind is the same defect with a longer fuse.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LIVE_RETRY_DELAYS_MS, openUsageLive, type UsageLiveController } from '$lib/usage-live';
import {
	frameText,
	liveRecorder,
	liveStreams,
	resetVisibility,
	setVisibility,
	settle,
	stubLiveFetch
} from '../support/live-stream';

function frame(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return { active: [], recent: [], error_provider: '', ...overrides };
}

function initOf(mock: { mock: { calls: unknown[][] } }, index: number): RequestInit {
	return (mock.mock.calls[index][1] ?? {}) as RequestInit;
}

let controller: UsageLiveController | null = null;

beforeEach(() => {
	vi.useFakeTimers();
	setVisibility('visible');
});

afterEach(() => {
	controller?.stop();
	controller = null;
	resetVisibility();
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('openUsageLive stop conditions', () => {
	it('stops reading while paused and reads again on resume', async () => {
		const { answer, streams } = liveStreams();
		const mock = stubLiveFetch([answer]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();
		streams[0].send(frameText(frame()));
		await settle();

		controller.pause();
		expect(initOf(mock, 0).signal?.aborted).toBe(true);
		expect(recorded.reports.at(-1)).toEqual({ status: 'paused', reason: null, retrying: false });

		await vi.advanceTimersByTimeAsync(600_000);
		expect(mock.mock.calls).toHaveLength(1);

		controller.resume();
		await settle();
		expect(mock.mock.calls).toHaveLength(2);
		expect(recorded.reports.at(-1)?.status).toBe('connecting');
	});

	it('does not schedule a retry while paused', async () => {
		const mock = stubLiveFetch([() => new Response('nope', { status: 500 })]);
		const { handlers } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();

		controller.pause();
		await vi.advanceTimersByTimeAsync(600_000);

		expect(mock.mock.calls).toHaveLength(1);
	});

	it('stops reading while the tab is hidden and reads again when it returns', async () => {
		const { answer, streams } = liveStreams();
		const mock = stubLiveFetch([answer]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();
		streams[0].send(frameText(frame()));
		await settle();
		expect(recorded.reports.at(-1)?.status).toBe('live');

		setVisibility('hidden');
		expect(initOf(mock, 0).signal?.aborted).toBe(true);
		expect(recorded.reports.at(-1)?.status).toBe('idle');

		await vi.advanceTimersByTimeAsync(600_000);
		expect(mock.mock.calls).toHaveLength(1);

		setVisibility('visible');
		await settle();
		expect(mock.mock.calls).toHaveLength(2);
	});

	it('reads nothing before the tab has ever been visible', async () => {
		setVisibility('hidden');
		const mock = stubLiveFetch([() => new Response('', { status: 200 })]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();

		expect(mock.mock.calls).toEqual([]);
		expect(recorded.reports).toEqual([{ status: 'idle', reason: null, retrying: false }]);

		setVisibility('visible');
		await settle();
		expect(mock.mock.calls).toHaveLength(1);
	});

	it('does not mint retries by hiding and showing the tab', async () => {
		const mock = stubLiveFetch([() => new Response('nope', { status: 500 })]);
		const { handlers } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();

		// One retry spent, then the tab goes away and comes back. The return is one attempt, and the budget
		// it draws on is the same one: an operator cannot mint attempts by toggling tabs.
		await vi.advanceTimersByTimeAsync(LIVE_RETRY_DELAYS_MS[0]);
		const afterOneRetry = mock.mock.calls.length;

		setVisibility('hidden');
		setVisibility('visible');
		await settle();
		expect(mock.mock.calls.length).toBe(afterOneRetry + 1);

		await vi.advanceTimersByTimeAsync(600_000);
		expect(mock.mock.calls).toHaveLength(1 + LIVE_RETRY_DELAYS_MS.length);
	});

	it('releases the connection and the listener when the screen goes away', async () => {
		const { answer } = liveStreams();
		const mock = stubLiveFetch([answer]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();

		controller.stop();
		controller = null;
		expect(initOf(mock, 0).signal?.aborted).toBe(true);

		const reports = recorded.reports.length;
		setVisibility('hidden');
		setVisibility('visible');
		await settle();

		expect(mock.mock.calls).toHaveLength(1);
		expect(recorded.reports).toHaveLength(reports);
	});
});
