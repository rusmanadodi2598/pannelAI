// Reading the frames out of a live Usage response body (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F2).
//
// Separated from the connection owner in `usage-live.ts` for the same reason the playground separates its
// reader: this half is about read boundaries and frame shapes, and that half is about when to connect,
// when to stop, and what to say when it fails. A test of one should not have to arrange the other.
//
// Nothing here touches a clock, a socket, or the DOM. It takes a response, a signal, and a callback, and it
// answers with how the read ended.

import { readSseData } from '$lib/schemas/playground-stream';
import { schemaUsageLiveFrame, type UsageLiveFrame } from '$lib/schemas/usage-live';

export type LiveReadOutcome =
	/** The gateway closed the stream. */
	| { outcome: 'ended' }
	/** The caller aborted the read, which is a stop rather than a failure. */
	| { outcome: 'aborted' }
	| { outcome: 'failed'; reason: string };

function parseFrame(data: string): UsageLiveFrame | null {
	try {
		return schemaUsageLiveFrame.parse(JSON.parse(data));
	} catch {
		// A frame the panel cannot read is dropped rather than ending the read: one malformed frame is not a
		// dead connection, and the next frame may be readable.
		return null;
	}
}

/**
 * Reads frames until the stream ends, breaks, or is aborted.
 *
 * `readSseData` holds the buffer splitter, so a frame that arrives across two network reads is assembled
 * before it is parsed and a keepalive comment is skipped rather than mistaken for one. Both are its own
 * tested rules; this function is the loop around them.
 */
export async function readLiveFrames(
	response: Response,
	signal: AbortSignal,
	onFrame: (frame: UsageLiveFrame) => void
): Promise<LiveReadOutcome> {
	if (response.body === null) {
		return { outcome: 'failed', reason: 'The live stream answered without a body.' };
	}

	const reader = response.body.getReader();
	const decoder = new TextDecoder();
	let buffer = '';

	try {
		for (;;) {
			const { done, value } = await reader.read();
			if (done) break;

			buffer += decoder.decode(value, { stream: true });
			const read = readSseData(buffer);
			buffer = read.rest;

			for (const data of read.data) {
				const frame = parseFrame(data);
				if (frame !== null) onFrame(frame);
			}
		}
	} catch (error) {
		if (signal.aborted) return { outcome: 'aborted' };
		return {
			outcome: 'failed',
			reason: error instanceof Error ? error.message : 'The live stream could not be read.'
		};
	}

	return { outcome: 'ended' };
}
