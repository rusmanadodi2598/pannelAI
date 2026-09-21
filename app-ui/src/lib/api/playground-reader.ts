// Reading a streamed answer off the wire (docs/SPEC-UI/001-SPEC-UI.md §6.15).
//
// This is the half of the send that knows nothing about fetch: a response body goes in and the answer
// plus the facts come out through the caller's handlers. It is a module of its own because its subject is
// read boundaries rather than requests, and the frame parser is the one piece of this screen that has to
// survive a network which splits wherever it likes.
//
// Two decisions are worth stating. A frame the chunk schema does not recognize is skipped rather than
// fatal, because the raw frames are on screen for a reader who wants to see them. And a stream that ends
// without the sentinel is reported as truncated, which is the honest description of an answer that
// stopped mid-sentence: it is neither a success nor a failure the far end announced.

import {
	EMPTY_ANSWER,
	SSE_DONE,
	applyChunk,
	readSseData,
	type AnswerState
} from '$lib/schemas/playground-stream';
import {
	schemaChatChunk,
	type PlaygroundErrorCode,
	type PlaygroundFailure
} from '$lib/schemas/playground';

/** Why a stream stopped, which is not the same as whether it succeeded. */
export type PlaygroundEndReason = 'done' | 'stopped' | 'truncated';

export type PlaygroundStreamHandlers = {
	/** The status the panel's route answered with, once the answer is known to be a stream. */
	onStart?: (status: number) => void;
	/** The answer so far, plus the frames as they arrived, for the raw-body disclosure. */
	onUpdate: (answer: AnswerState, raw: string) => void;
	onEnd: (reason: PlaygroundEndReason) => void;
	onFailure: (failure: PlaygroundFailure) => void;
};

/** Reads the SSE body, folding each frame into the answer. */
export async function readStream(
	response: Response,
	handlers: PlaygroundStreamHandlers,
	signal?: AbortSignal
): Promise<void> {
	const reader = response.body?.getReader();
	if (!reader) {
		handlers.onFailure(localFailure('INTERNAL_ERROR', 'The answer carried no readable body.'));
		return;
	}

	const decoder = new TextDecoder();
	let buffer = '';
	let raw = '';
	let answer = EMPTY_ANSWER;

	try {
		for (;;) {
			const { value, done } = await reader.read();
			if (done) break;

			const text = decoder.decode(value, { stream: true });
			raw += text;
			buffer += text;

			const read = readSseData(buffer);
			buffer = read.rest;

			for (const data of read.data) {
				if (data === SSE_DONE) {
					handlers.onUpdate(answer, raw);
					handlers.onEnd('done');
					return;
				}

				const chunk = parseChunk(data);
				if (chunk !== null) answer = applyChunk(answer, chunk);
			}

			handlers.onUpdate(answer, raw);
		}
	} catch (cause) {
		// An abort is the operator's own Stop, not a failure: whatever text arrived stays on screen.
		if (isAbort(cause) || signal?.aborted === true) {
			handlers.onEnd('stopped');
			return;
		}
		handlers.onFailure(localFailure('GATEWAY_ERROR', 'The answer stopped before it finished.'));
		return;
	}

	handlers.onEnd('truncated');
}

/** One frame's JSON, or null when the frame is not a chunk this screen models. */
function parseChunk(data: string) {
	try {
		const parsed = schemaChatChunk.safeParse(JSON.parse(data));
		return parsed.success ? parsed.data : null;
	} catch {
		return null;
	}
}

/** A failure the panel itself names, with no gateway object behind it. */
export function localFailure(code: PlaygroundErrorCode, message: string): PlaygroundFailure {
	return { error: { code, message } };
}

/**
 * Whether a thrown value is an abort.
 *
 * The check reads `name` rather than testing the class, because an aborted fetch rejects with a
 * `DOMException` in some runtimes and an `Error` in others, and both spell the name the same way.
 */
export function isAbort(cause: unknown): boolean {
	if (typeof cause !== 'object' || cause === null) return false;
	return (cause as { name?: unknown }).name === 'AbortError';
}
