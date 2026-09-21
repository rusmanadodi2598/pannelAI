// Playground render tests: the send and the streamed answer (docs/SPEC-UI/001-SPEC-UI.md §6.15).
//
// What the stream reader does with bytes is `tests/api/playground-reader.test.ts`; this file is about what
// the operator ends up looking at. §6.15 rule 4 asks for the resolved model and the upstream status beside
// the answer so a failure is diagnosable, and the facts row is asserted value by value, including the two
// values that are absent from the wire and read as not stated rather than as a zero this panel made up.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import PlaygroundPage from '../../src/routes/playground/+page.svelte';
import { DONE_FRAME, USAGE_FRAME, chunkFrame, streamResponse } from '../support/playground-sse';
import { stallingAnswer, stubPlayground } from '../support/playground-page';

const MODELS = { data: [{ id: 'deepseek/chat', owned_by: 'deepseek' }] };
const READ_FROM = 'Read from GET /playground/models.';

/** The answer block, which is a section headed by the copy's own word. */
function answerBlock(): HTMLElement {
	const heading = screen.getByRole('heading', { name: 'Answer' });
	return heading.closest('section') as HTMLElement;
}

async function sendMessage(text: string): Promise<void> {
	await screen.findByText(READ_FROM);
	await fireEvent.input(screen.getByLabelText('Message'), { target: { value: text } });
	await fireEvent.click(screen.getByRole('button', { name: 'Send' }));
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('PlaygroundPage send', () => {
	it('sends the chosen model and message, and renders the answer with its facts', async () => {
		const calls = stubPlayground({
			models: MODELS,
			chat: () => streamResponse([chunkFrame('Hel'), chunkFrame('lo'), USAGE_FRAME, DONE_FRAME])
		});
		render(PlaygroundPage);

		await sendMessage('hello');

		expect(await screen.findByText('Hello')).toBeTruthy();
		expect(calls.chatBodies[0]).toEqual({ model: 'deepseek/chat', message: 'hello' });

		const block = within(answerBlock());
		expect(
			block.getByText('The stream ended with the sentinel the data plane documents.')
		).toBeTruthy();

		// The facts: the resolved model and the usage the wire reported, and the two it did not state.
		expect(block.getByText('deepseek/chat')).toBeTruthy();
		expect(block.getByText('4 prompt, 2 completion, 6 total')).toBeTruthy();
		expect(block.getByText('200')).toBeTruthy();
		expect(block.getAllByText('not stated').length).toBe(1);

		// The raw frames stay available, exactly as they arrived.
		expect(block.getByText(/data: \[DONE\]/)).toBeTruthy();
	});

	it('stops the stream on request and keeps the text that had arrived', async () => {
		stubPlayground({ models: MODELS, chat: stallingAnswer('partial') });
		render(PlaygroundPage);

		await sendMessage('hi');

		expect(await screen.findByText('partial')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Stop' }));

		expect(await screen.findByText(/You stopped the stream/)).toBeTruthy();
		expect(within(answerBlock()).getByText('partial')).toBeTruthy();
	});

	it('reports a refused send with the gateway own code, and renders no answer', async () => {
		const refusal = {
			error: {
				code: 'GATEWAY_ERROR',
				message: 'The gateway routes no model by that string.',
				gateway: {
					code: 'MODEL_NOT_FOUND',
					type: 'invalid_request_error',
					message: 'The gateway routes no model by that string.'
				}
			}
		};

		stubPlayground({
			models: MODELS,
			chat: () =>
				new Response(JSON.stringify(refusal), {
					status: 502,
					headers: { 'content-type': 'application/json' }
				})
		});
		render(PlaygroundPage);

		await sendMessage('hi');

		expect(await screen.findByText('The send failed')).toBeTruthy();
		expect(screen.getByText(/The gateway wrote MODEL_NOT_FOUND/)).toBeTruthy();
		expect(screen.queryByRole('heading', { name: 'Answer' })).toBeNull();
	});

	it('keeps a partial answer on screen when the stream breaks', async () => {
		// The frame is delivered by the first read and the break comes on the second, because a stream that
		// errors with a frame still queued drops that frame: the point of the row is text that did arrive.
		let reads = 0;
		const body = new ReadableStream<Uint8Array>({
			pull(controller) {
				if (reads === 0) {
					reads += 1;
					controller.enqueue(new TextEncoder().encode(chunkFrame('half')));
					return;
				}
				controller.error(new Error('socket closed'));
			}
		});

		stubPlayground({
			models: MODELS,
			chat: () =>
				new Response(body, { status: 200, headers: { 'content-type': 'text/event-stream' } })
		});
		render(PlaygroundPage);

		await sendMessage('hi');

		expect(await screen.findByText('The send failed')).toBeTruthy();
		const block = within(answerBlock());
		expect(block.getByText('half')).toBeTruthy();
		expect(block.getByText(/The stream stopped before it reached an end/)).toBeTruthy();
	});
});
