// Playground copy (docs/SPEC-UI/001-SPEC-UI.md §6.15).
//
// English only, no em dash (R-02), no marketing vocabulary (R-16), and no figure the wire did not provide
// (R-17): the model string, the HTTP status, and the token counts are passed in, never composed here.
//
// One sentence is deliberately absent. The unavailable state does not name the panel's environment
// variable itself, because the panel's route is what knows it and its message carries the name, so the
// screen renders the sentence it was given. Naming it here would put the variable in the browser bundle,
// which is what §6.15 rule 1 forbids and what tests/server/playground-secret.test.ts enforces.

import {
	playgroundFailureSentence,
	type ChatUsage,
	type PlaygroundFailure
} from '$lib/schemas/playground';

export const PLAYGROUND_COPY = {
	title: 'Playground Chat',
	subtitle: 'Send one message and read what the gateway answers.',

	models: {
		loading: 'Loading the models this key routes',
		errorTitle: 'Could not read the model list',
		retry: 'Try again',
		/** The key is unset. The description the screen renders is the panel's own sentence, which names it. */
		unavailableTitle: 'The panel has no gateway key',
		unavailableNote: 'This screen never holds a key.',
		emptyTitle: 'This key routes no model',
		emptyDescription: 'The gateway listed no models, so there is nothing to send.',
		/** What was read, stated as a fact about this screen rather than a claim about the gateway. */
		readFrom: (path: string) => `Read from GET ${path}.`,
		count: (count: number) => (count === 1 ? '1 model' : `${count} models`)
	},

	composer: {
		modelLabel: 'Model',
		modelNote: 'The model id exactly as the gateway lists it.',
		messageLabel: 'Message',
		messagePlaceholder: 'Write one message to send.',
		send: 'Send',
		stop: 'Stop',
		/** The cost, stated before the send rather than reported after it (§6.15 rule 4). */
		cost: 'This calls a real provider, uses quota, and is logged.'
	},

	answer: {
		heading: 'Answer',
		waiting: 'Waiting for the first token',
		streaming: 'Streaming the answer',
		/** A stream that carried no text at all, which is a fact about the answer rather than an error. */
		empty: 'The answer came back with no text.',
		facts: {
			model: 'Resolved model',
			finish: 'Finish reason',
			tokens: 'Tokens',
			status: 'HTTP status',
			notStated: 'not stated',
			notReported: 'not reported'
		},
		end: {
			done: 'The answer ended cleanly.',
			stopped: 'You stopped it. The text above is what arrived.',
			truncated: 'The answer stopped early, so the text above may be partial.'
		},
		/** A stream that broke mid-read: the failure block above says why, and this says what is on screen. */
		interrupted: 'The answer never finished. The text above is what arrived.',
		raw: {
			summary: 'Raw frames',
			intro: 'The frames exactly as they arrived.'
		}
	},

	failure: {
		title: 'The send failed',
		/** The machine code a developer is looking for, kept beside the sentence rather than instead of it. */
		gateway: (code: string, type: string) => `The gateway wrote ${code} (${type}).`
	}
} as const;

/**
 * The token counts as one line.
 *
 * The wire may report any subset of the three, so only what arrived is printed and a usage block with
 * nothing in it reads as unreported rather than as zeroes the panel made up.
 */
export function usageSentence(usage: ChatUsage | null): string {
	if (usage === null) return PLAYGROUND_COPY.answer.facts.notReported;

	const parts: string[] = [];
	if (usage.prompt_tokens !== undefined) parts.push(`${usage.prompt_tokens} prompt`);
	if (usage.completion_tokens !== undefined) parts.push(`${usage.completion_tokens} completion`);
	if (usage.total_tokens !== undefined) parts.push(`${usage.total_tokens} total`);

	return parts.length > 0 ? parts.join(', ') : PLAYGROUND_COPY.answer.facts.notReported;
}

/** A failure as one paragraph: the panel's own sentence, then the gateway's machine code when there is one. */
export function playgroundFailureDetail(failure: PlaygroundFailure): string {
	const sentence = playgroundFailureSentence(failure);
	const gateway = failure.error.gateway;

	if (!gateway) return sentence;

	return `${sentence} ${PLAYGROUND_COPY.failure.gateway(gateway.code, gateway.type)}`;
}
