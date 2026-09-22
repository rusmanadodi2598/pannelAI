// Usage live connection tests (src/lib/usage-live.ts, draft 012 F2).
//
// The subject is the connection and its failure policy: what it asks for, what it says when the gateway
// refuses, and how it retries. The rows are about the difference between failures an operator can act on
// (a route that is not served, a session that was refused) and the ones it cannot, which is why each has
// its own sentence. How frames are read is `usage-live-reader.test.ts`, and the stop conditions are
// `usage-live-stop.test.ts`.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	LIVE_RETRY_DELAYS_MS,
	USAGE_LIVE_PATH,
	openUsageLive,
	type UsageLiveController
} from '$lib/usage-live';
import {
	frameText,
	liveRecorder,
	liveStreams,
	resetVisibility,
	setVisibility,
	settle,
	spendRetryBudget,
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

describe('openUsageLive', () => {
	it('reads the live route and reports live once a frame arrives', async () => {
		const { answer, streams } = liveStreams();
		const mock = stubLiveFetch([answer]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();

		expect(mock.mock.calls[0][0]).toBe(USAGE_LIVE_PATH);
		expect(initOf(mock, 0).headers).toEqual({ accept: 'text/event-stream' });
		expect(recorded.reports[0]).toEqual({ status: 'connecting', reason: null, retrying: false });

		streams[0].send(
			frameText(frame({ active: [{ provider_id: 'openai', started_at: '2026-09-22T10:00:00Z' }] }))
		);
		await settle();

		expect(recorded.frames).toHaveLength(1);
		expect(recorded.frames[0].frame.active?.[0].provider_id).toBe('openai');
		expect(recorded.frames[0].receivedAt).toBeTypeOf('number');
		expect(recorded.reports.at(-1)).toEqual({ status: 'live', reason: null, retrying: false });
	});

	it('states that the route is not served and schedules a retry', async () => {
		const mock = stubLiveFetch([() => new Response('not found', { status: 404 })]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();

		expect(recorded.reports.at(-1)).toEqual({
			status: 'unavailable',
			reason: 'The gateway has no live stream route yet (it answered 404).',
			retrying: true
		});

		await vi.advanceTimersByTimeAsync(LIVE_RETRY_DELAYS_MS[0]);
		expect(mock.mock.calls).toHaveLength(2);
	});

	it('names a refused session rather than a generic failure', async () => {
		stubLiveFetch([() => new Response('', { status: 401 })]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();

		expect(recorded.reports.at(-1)?.reason).toBe('The live stream refused this session (401).');
	});

	it('stops retrying once the budget is spent, and does not read again on its own', async () => {
		const mock = stubLiveFetch([() => new Response('nope', { status: 500 })]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();
		await spendRetryBudget();

		expect(mock.mock.calls).toHaveLength(1 + LIVE_RETRY_DELAYS_MS.length);
		expect(recorded.reports.at(-1)).toEqual({
			status: 'unavailable',
			reason: 'The live stream answered 500.',
			retrying: false
		});

		await vi.advanceTimersByTimeAsync(600_000);
		expect(mock.mock.calls).toHaveLength(1 + LIVE_RETRY_DELAYS_MS.length);
	});

	it('reads again when the operator asks it to, from a fresh budget', async () => {
		const { answer, streams } = liveStreams();
		const mock = stubLiveFetch([() => new Response('nope', { status: 500 }), answer]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();
		await spendRetryBudget();
		const spent = mock.mock.calls.length;

		controller.retry();
		await settle();

		expect(mock.mock.calls).toHaveLength(spent + 1);
		expect(recorded.reports.at(-1)).toEqual({
			status: 'connecting',
			reason: null,
			retrying: false
		});

		streams[0].send(frameText(frame()));
		await settle();

		expect(recorded.reports.at(-1)).toEqual({ status: 'live', reason: null, retrying: false });
	});

	it('forgets the failure on the frame that follows it', async () => {
		const { answer, streams } = liveStreams();
		stubLiveFetch([answer]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();
		streams[0].send(frameText(frame()));
		await settle();

		streams[0].break(new Error('socket closed'));
		await settle();
		expect(recorded.reports.at(-1)).toEqual({
			status: 'unavailable',
			reason: 'socket closed',
			retrying: true
		});

		await vi.advanceTimersByTimeAsync(LIVE_RETRY_DELAYS_MS[0]);
		expect(recorded.reports.at(-1)).toEqual({
			status: 'connecting',
			reason: null,
			retrying: false
		});

		streams[1].send(frameText(frame()));
		await settle();
		expect(recorded.reports.at(-1)).toEqual({ status: 'live', reason: null, retrying: false });
	});

	it('reports a stream the gateway closed', async () => {
		const { answer, streams } = liveStreams();
		stubLiveFetch([answer]);
		const { handlers, recorded } = liveRecorder();

		controller = openUsageLive(handlers);
		await settle();
		streams[0].send(frameText(frame()));
		await settle();

		streams[0].close();
		await settle();

		expect(recorded.reports.at(-1)).toEqual({
			status: 'unavailable',
			reason: 'The gateway closed the live stream.',
			retrying: true
		});
	});
});
