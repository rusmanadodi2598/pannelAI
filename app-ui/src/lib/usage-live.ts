// The panel's owner of the Usage live connection (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F2).
//
// It reads the stream with `fetch` rather than with `EventSource`, and the reason is the failure mode. An
// `EventSource` retries on its own and never says why it failed: a route that does not exist (today's
// state, draft 012 F4) and a dropped connection look identical from the panel's side, so the screen could
// not state a cause. A fetch answers with a status, and the panel already reads a stream by fetch, so it
// has one streaming technique rather than two.
//
// This module owns the retry schedule, the stop conditions, and the sentence the screen shows. What a frame
// means is `usage-live-view.ts`, how frames are read is `usage-live-reader.ts`, and the drawing is neither.

import { readLiveFrames } from '$lib/usage-live-reader';
import type { UsageLiveFrame } from '$lib/schemas/usage-live';
import { streamStatus, type StreamStatus } from '$lib/schemas/usage-live-view';

/** The route draft 010 §10.3 asks the gateway to serve. Session-gated, so the request carries the cookie. */
export const USAGE_LIVE_PATH = '/api/v1/usage/live';

/**
 * The waits between attempts, in order.
 *
 * Bounded at both ends: the first retry is soon enough that a blip is invisible, and the last is long enough
 * that a gateway which is down is not asked ten times a minute by every open tab. After the last delay the
 * module stops and the screen offers the retry control instead: an unbounded loop is a load generator.
 */
export const LIVE_RETRY_DELAYS_MS = [1_000, 2_000, 4_000, 8_000, 15_000];

export type LiveReport = {
	status: StreamStatus;
	/** What happened, in one sentence, for a status the screen has to explain. Null while live. */
	reason: string | null;
	/** Whether another attempt is already scheduled. */
	retrying: boolean;
};

export type UsageLiveHandlers = {
	onFrame: (frame: UsageLiveFrame, receivedAt: number) => void;
	onReport: (report: LiveReport) => void;
};

export type UsageLiveController = {
	/** Stops reading until `resume`, and aborts the request in flight. */
	pause: () => void;
	/** Reads again from a fresh retry budget, because the operator asked for it. */
	resume: () => void;
	/** Reads again after the module gave up, from a fresh retry budget. */
	retry: () => void;
	/** Releases everything. The screen calls this when it is destroyed. */
	stop: () => void;
};

function describeStatus(status: number): string {
	if (status === 404) return 'The gateway has no live stream route yet (it answered 404).';
	if (status === 401 || status === 403) return `The live stream refused this session (${status}).`;
	return `The live stream answered ${status}.`;
}

/**
 * Opens the stream and keeps it open until the screen goes away. Every transition has a rule behind it: a
 * frame is the only thing that makes the panel live and a failure the only thing that makes it unavailable;
 * a pause outranks both, because while the operator has paused it the reason is the pause; and a hidden tab
 * stops the read for the reason §8.6.1 gives polling, reading once on the way back.
 */
export function openUsageLive(handlers: UsageLiveHandlers): UsageLiveController {
	let paused = false;
	let stopped = false;
	let fresh = false;
	let down = false;
	let active = false;
	let reason: string | null = null;
	let attempt = 0;
	let timer: ReturnType<typeof setTimeout> | null = null;
	let abort: AbortController | null = null;

	function report(): void {
		handlers.onReport({
			status: streamStatus({ paused, fresh, down, active }),
			reason,
			retrying: timer !== null
		});
	}

	function clearTimer(): void {
		if (timer === null) return;
		clearTimeout(timer);
		timer = null;
	}

	function halt(): void {
		clearTimer();
		abort?.abort();
		abort = null;
		active = false;
		report();
	}

	function schedule(): void {
		if (stopped || paused) return;
		if (attempt >= LIVE_RETRY_DELAYS_MS.length) return;

		const delay = LIVE_RETRY_DELAYS_MS[attempt];
		attempt += 1;
		timer = setTimeout(() => {
			timer = null;
			void run();
		}, delay);
	}

	function fail(message: string): void {
		down = true;
		reason = message;
		active = false;
		// Scheduled before the report: whether another attempt is coming is part of what the screen states,
		// and a report that lagged the schedule would say retrying had stopped.
		schedule();
		report();
	}

	function onFrame(frame: UsageLiveFrame): void {
		fresh = true;
		down = false;
		reason = null;
		attempt = 0;
		handlers.onFrame(frame, Date.now());
		report();
	}

	async function run(): Promise<void> {
		if (stopped || paused) return;

		if (document.visibilityState !== 'visible') {
			active = false;
			report();
			return;
		}

		clearTimer();
		active = true;
		// Reset per attempt, so `live` means "frames are arriving on the connection that is open now": a
		// reconnect is not live until it delivers one.
		fresh = false;
		report();

		const controller = new AbortController();
		abort = controller;

		try {
			const response = await fetch(USAGE_LIVE_PATH, {
				headers: { accept: 'text/event-stream' },
				credentials: 'same-origin',
				signal: controller.signal
			});

			if (!response.ok) {
				fail(describeStatus(response.status));
				return;
			}

			// The gateway accepted the stream, which is what a fetch can say that an `EventSource` cannot: the
			// connection is up, though nothing is live until a frame arrives. The failure that led to this
			// attempt is forgotten, and a second failure states its own reason.
			down = false;
			reason = null;
			report();

			const read = await readLiveFrames(response, controller.signal, onFrame);
			if (read.outcome === 'aborted') return;
			if (read.outcome === 'failed') {
				fail(read.reason);
				return;
			}
			fail('The gateway closed the live stream.');
		} catch (error) {
			// An abort is this module's own doing (a pause, a hidden tab, or teardown), not a failure to state.
			if (controller.signal.aborted) return;
			fail(error instanceof Error ? error.message : 'The live stream could not be read.');
		} finally {
			if (abort === controller) abort = null;
		}
	}

	function onVisibility(): void {
		if (document.visibilityState !== 'visible') {
			halt();
			return;
		}
		if (stopped || paused) return;
		void run();
	}

	document.addEventListener('visibilitychange', onVisibility);
	void run();

	return {
		pause(): void {
			if (stopped || paused) return;
			paused = true;
			halt();
		},
		resume(): void {
			if (stopped || !paused) return;
			paused = false;
			attempt = 0;
			void run();
		},
		retry(): void {
			if (stopped || paused) return;
			attempt = 0;
			down = false;
			reason = null;
			void run();
		},
		stop(): void {
			stopped = true;
			document.removeEventListener('visibilitychange', onVisibility);
			clearTimer();
			abort?.abort();
			abort = null;
			active = false;
		}
	};
}
