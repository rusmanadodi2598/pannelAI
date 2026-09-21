// Stream fixtures shared by the playground's browser-side tests.
//
// The three files that read a stream need the same pieces: a body that hands the reader exactly the
// chunks it was given (so a split frame stays split), the frames a data plane sends, and a recorder for
// what the reader reported. They live in `support/` rather than in one test file because a fixture inside
// a test file cannot be imported without importing its tests.

import type { PlaygroundStreamHandlers } from '$lib/api/playground';
import type { PlaygroundFailure } from '$lib/schemas/playground';
import type { AnswerState } from '$lib/schemas/playground-stream';

const encoder = new TextEncoder();

/** A body that arrives in the pieces it was given, one read per piece. */
export function sseBody(chunks: string[]): ReadableStream<Uint8Array> {
	return new ReadableStream({
		start(controller) {
			for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
			controller.close();
		}
	});
}

export function streamResponse(chunks: string[]): Response {
	return new Response(sseBody(chunks), {
		status: 200,
		headers: { 'content-type': 'text/event-stream' }
	});
}

/** One streamed chunk frame carrying text, with the model the gateway resolved the request to. */
export function chunkFrame(content: string, model = 'deepseek/chat'): string {
	return `data: ${JSON.stringify({ model, choices: [{ delta: { content } }] })}\n\n`;
}

/** The sentinel that ends a stream. It is data rather than JSON, which is why it is checked by name. */
export const DONE_FRAME = 'data: [DONE]\n\n';

/** The final frame of a stream that asked for usage: no choice, and the only token counts on the wire. */
export const USAGE_FRAME = `data: ${JSON.stringify({
	model: 'deepseek/chat',
	choices: [],
	usage: { prompt_tokens: 4, completion_tokens: 2, total_tokens: 6 }
})}\n\n`;

export type Recorded = {
	starts: number[];
	updates: AnswerState[];
	ends: string[];
	failures: PlaygroundFailure[];
	raw: string[];
};

export function streamHandlers(): { handlers: PlaygroundStreamHandlers; recorded: Recorded } {
	const recorded: Recorded = { starts: [], updates: [], ends: [], failures: [], raw: [] };

	return {
		recorded,
		handlers: {
			onStart: (status) => recorded.starts.push(status),
			onUpdate: (answer, raw) => {
				recorded.updates.push(answer);
				recorded.raw.push(raw);
			},
			onEnd: (reason) => recorded.ends.push(reason),
			onFailure: (failure) => recorded.failures.push(failure)
		}
	};
}
