// Reading a streamed playground answer (docs/SPEC-UI/001-SPEC-UI.md §6.15).
//
// The screen streams, so the answer arrives as server-sent events and has to be turned back into text
// plus the facts a reader needs. Two pure pieces do that, and they are separate from the wire shapes in
// `playground.ts` because their tests are about chunk boundaries rather than about fields: a buffer
// splitter that holds for any network read, and a folder that turns frames into an answer.
//
// Both are pure functions of their input. Nothing here touches fetch, a store, or the DOM.

import type { ChatChunk, ChatUsage } from './playground';

/** The sentinel that ends an OpenAI stream. It is data, not JSON, which is why it is checked by name. */
export const SSE_DONE = '[DONE]';

/** What a read of the byte stream produced: the frames it completed, and the bytes of the one it did not. */
export type SseRead = {
	data: string[];
	rest: string;
};

/**
 * Reads every complete server-sent event out of a buffer.
 *
 * The reader is a buffer splitter rather than a line parser, because a network read lands wherever it
 * lands: a frame may arrive in three chunks, and two frames may arrive in one. A frame is complete at
 * its blank line, its `data:` lines are joined with a newline as the SSE format requires, and whatever
 * follows the last blank line is returned untouched so the caller can prepend the next read to it.
 */
export function readSseData(buffer: string): SseRead {
	const data: string[] = [];
	const normalized = buffer.replace(/\r\n/g, '\n');
	const parts = normalized.split('\n\n');
	const rest = parts.pop() ?? '';

	for (const frame of parts) {
		const lines: string[] = [];
		for (const line of frame.split('\n')) {
			if (!line.startsWith('data:')) continue;
			lines.push(line.slice('data:'.length).replace(/^ /, ''));
		}
		if (lines.length > 0) data.push(lines.join('\n'));
	}

	return { data, rest };
}

/** The answer as the screen accumulates it. Text grows; the other three are facts that arrive once. */
export type AnswerState = {
	text: string;
	model: string | null;
	finishReason: string | null;
	usage: ChatUsage | null;
};

export const EMPTY_ANSWER: AnswerState = {
	text: '',
	model: null,
	finishReason: null,
	usage: null
};

/**
 * Folds one streamed frame into the answer.
 *
 * The rules are the wire's own: content concatenates across frames, the model and the finish reason are
 * facts stated once (the first non-empty wins, so a later frame cannot overwrite the resolved model
 * with a placeholder), and usage is read from whichever frame carries it, which is the last one.
 */
export function applyChunk(state: AnswerState, chunk: ChatChunk): AnswerState {
	const choice = chunk.choices[0];
	const content = choice?.delta?.content ?? '';
	const finish = choice?.finish_reason ?? null;

	return {
		text: content.length > 0 ? state.text + content : state.text,
		model: state.model ?? chunk.model ?? null,
		finishReason: state.finishReason ?? finish,
		usage: chunk.usage ?? state.usage
	};
}
