// The playground page's fetch stub.
//
// The page calls two of the panel's own routes, so one stub answers both and records what was asked. The
// stalling answer is here because it is the only way to observe a stream that is still running: it emits
// one frame and then stays open until the signal aborts it, which is what a real socket does when the
// operator presses Stop.

import { vi } from 'vitest';
import { DONE_FRAME, chunkFrame, streamResponse } from './playground-sse';

const encoder = new TextEncoder();

export type PageCalls = {
	models: number;
	chatBodies: unknown[];
	signals: (AbortSignal | undefined)[];
};

export type ChatAnswer = (init?: RequestInit) => Response;

/** Answers both playground routes, and counts the reads so a retry can be observed. */
export function stubPlayground(options: {
	models: unknown;
	modelsStatus?: number;
	/** The answer to a send. Defaults to a stream that ends immediately. */
	chat?: ChatAnswer;
}): PageCalls {
	const calls: PageCalls = { models: 0, chatBodies: [], signals: [] };

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);

		if (url === '/playground/models') {
			calls.models += 1;
			return new Response(JSON.stringify(options.models), {
				status: options.modelsStatus ?? 200,
				headers: { 'content-type': 'application/json' }
			});
		}

		calls.chatBodies.push(init?.body === undefined ? null : JSON.parse(String(init.body)));
		calls.signals.push(init?.signal ?? undefined);

		return options.chat ? options.chat(init) : streamResponse([DONE_FRAME]);
	});

	return calls;
}

/** A stream that emits one frame and stays open until the caller aborts it. */
export function stallingAnswer(text = 'partial'): ChatAnswer {
	return (init) => {
		const body = new ReadableStream<Uint8Array>({
			start(controller) {
				controller.enqueue(encoder.encode(chunkFrame(text)));
				init?.signal?.addEventListener('abort', () => {
					controller.error(new DOMException('stopped', 'AbortError'));
				});
			}
		});

		return new Response(body, { status: 200, headers: { 'content-type': 'text/event-stream' } });
	};
}
