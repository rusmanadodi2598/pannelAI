// Live stream fixtures for the usage reader's tests (src/lib/usage-live.ts).
//
// The reader is driven by a `fetch` stub whose body the test pushes into, so a test can say exactly when
// a frame arrives, when the gateway closes the stream, and when a read breaks. That control is the point:
// the behaviour under test is what the reader does *between* frames, which a body that arrives all at once
// could not exercise.

import { vi } from 'vitest';
import type { UsageLiveFrame } from '$lib/schemas/usage-live';
import { LIVE_RETRY_DELAYS_MS, type LiveReport, type UsageLiveHandlers } from '$lib/usage-live';

const encoder = new TextEncoder();

/** One frame as the gateway writes it, which is what `readSseData` reads back. */
export function frameText(frame: unknown): string {
	return `data: ${JSON.stringify(frame)}\n\n`;
}

/** A keepalive comment, which the reader must never mistake for a frame. */
export const PING_TEXT = ': ping\n\n';

export type LiveBody = {
	body: ReadableStream<Uint8Array>;
	send: (text: string) => void;
	close: () => void;
	/** Breaks the read, the way a socket reset does. */
	break: (error: Error) => void;
};

export function liveBody(): LiveBody {
	let controller: ReadableStreamDefaultController<Uint8Array> | null = null;
	let ended = false;

	const body = new ReadableStream<Uint8Array>({
		start(next) {
			controller = next;
		}
	});

	function guard(): ReadableStreamDefaultController<Uint8Array> | null {
		if (ended || controller === null) return null;
		return controller;
	}

	return {
		body,
		send(text) {
			guard()?.enqueue(encoder.encode(text));
		},
		close() {
			const next = guard();
			if (next === null) return;
			ended = true;
			next.close();
		},
		break(error) {
			const next = guard();
			if (next === null) return;
			ended = true;
			next.error(error);
		}
	};
}

export type LiveRecorded = {
	reports: LiveReport[];
	frames: { frame: UsageLiveFrame; receivedAt: number }[];
};

export function liveRecorder(): { handlers: UsageLiveHandlers; recorded: LiveRecorded } {
	const recorded: LiveRecorded = { reports: [], frames: [] };

	return {
		recorded,
		handlers: {
			onFrame: (frame, receivedAt) => recorded.frames.push({ frame, receivedAt }),
			onReport: (report) => recorded.reports.push(report)
		}
	};
}

export type LiveAnswer = () => Response | Promise<Response>;

/**
 * A `fetch` stub that answers each call with the next answer in the list.
 *
 * The last answer repeats, so a test about the retry budget can hand over one failing answer and let
 * every attempt fail, while a test about recovery can hand over a failure and then a working stream.
 */
export function stubLiveFetch(answers: LiveAnswer[]) {
	const mock = vi.fn(async (_input: unknown, _init?: RequestInit) => {
		const answer = answers[Math.min(mock.mock.calls.length - 1, answers.length - 1)];
		return answer();
	});

	vi.stubGlobal('fetch', mock);
	return mock;
}

export function liveResponse(body: ReadableStream<Uint8Array>, status = 200): Response {
	return new Response(body, { status, headers: { 'content-type': 'text/event-stream' } });
}

/**
 * An answer that builds a fresh stream per call, and the streams it built.
 *
 * A retry has to be given a body of its own: a `Response` wraps a stream once, and handing the same one
 * to a second attempt fails before the reader sees anything, which would make a test about recovery pass
 * or fail for a reason that has nothing to do with the reader.
 */
export function liveStreams(): { answer: LiveAnswer; streams: LiveBody[] } {
	const streams: LiveBody[] = [];

	return {
		streams,
		answer: () => {
			const stream = liveBody();
			streams.push(stream);
			return liveResponse(stream.body);
		}
	};
}

/** Points the document's visibility at a state and fires the event a browser would fire. */
export function setVisibility(state: string): void {
	Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
	document.dispatchEvent(new Event('visibilitychange'));
}

export function resetVisibility(): void {
	delete (document as { visibilityState?: string }).visibilityState;
}

/**
 * Lets every promise the reader chained settle, without moving the clock.
 *
 * The reader's work between two frames is all microtasks, so a test that pushed a frame in has to let them
 * run before asserting. Fake timers mean the microtask queue is not drained by waiting, which is why this
 * is `advanceTimersByTimeAsync` rather than a real pause.
 */
export async function settle(): Promise<void> {
	await vi.advanceTimersByTimeAsync(0);
}

/** Spends the whole retry budget, so a test can reach the state where the reader has given up. */
export async function spendRetryBudget(): Promise<void> {
	for (const delay of LIVE_RETRY_DELAYS_MS) await vi.advanceTimersByTimeAsync(delay);
}
