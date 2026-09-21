// Playground wire shapes (docs/SPEC-UI/001-SPEC-UI.md §6.15 over docs/SPEC-API/001-SPEC-API.md §7.15).
//
// Three contracts meet on this screen and they are deliberately not the same contract:
//
//   The panel's own routes (`/playground/models`, `/playground/chat`) are strict, because the panel owns
//   both ends of them: an unknown field there is a mistake in this codebase, not a newer client.
//
//   The gateway's answer is tolerant, because it is a §7.15 response and §7.4 keeps an additive field
//   from breaking a screen.
//
//   The data-plane error envelope is a third shape, `{error:{message,type,code}}`, and its code set is
//   not published as closed: SPEC-API §8 names only PROVIDER_NOT_ROUTABLE as data-plane. So the code
//   crosses as a string, a known one gets a fallback sentence, and an unknown one is rendered rather
//   than dropped (§14 Q25).

import { z } from 'zod';

/**
 * The browser's request to the panel's own route.
 *
 * `model` carries the gateway's own bound (SPEC-API §7.15 caps the model string at 200 characters), so
 * a paste the gateway would refuse is refused here with the field named. The message has no cap of its
 * own: the gateway bounds the whole body at 8 MiB and a per-message limit would be a number this panel
 * invented.
 */
export const schemaPlaygroundRequest = z.strictObject({
	model: z.string().trim().min(1, 'Choose a model.').max(200),
	message: z.string().trim().min(1, 'Write a message to send.')
});

export type PlaygroundRequest = z.infer<typeof schemaPlaygroundRequest>;

/**
 * The gateway's models list (§7.15), narrowed to what the screen reads.
 *
 * Only `id` is required: it is the string the send control puts on the wire, and everything else is
 * display metadata the panel does not need. `owned_by` separates a provider model from a combo, which
 * is worth showing, so it is carried when present.
 */
export const schemaPlaygroundModel = z.object({
	id: z.string().min(1),
	owned_by: z.string().optional()
});

export const schemaPlaygroundModels = z.object({
	data: z.array(schemaPlaygroundModel)
});

export type PlaygroundModel = z.infer<typeof schemaPlaygroundModel>;
export type PlaygroundModels = z.infer<typeof schemaPlaygroundModels>;

/** The OpenAI usage block, every member optional: the panel reports what arrived, never a zero it made up. */
export const schemaChatUsage = z.object({
	prompt_tokens: z.number().optional(),
	completion_tokens: z.number().optional(),
	total_tokens: z.number().optional()
});

export type ChatUsage = z.infer<typeof schemaChatUsage>;

/**
 * One streamed OpenAI frame (§7.15), narrowed to what the screen renders.
 *
 * `choices` may be empty, and that is not a defect: the final frame of a stream that asked for usage
 * carries no choice, which is exactly how OpenAI documents it.
 */
export const schemaChatChunk = z.object({
	model: z.string().optional(),
	choices: z.array(
		z.object({
			delta: z.object({ content: z.string().optional() }).optional(),
			finish_reason: z.string().nullable().optional()
		})
	),
	usage: schemaChatUsage.nullable().optional()
});

export type ChatChunk = z.infer<typeof schemaChatChunk>;

/**
 * The data-plane error envelope (§4).
 *
 * Every member is carried as a string rather than a literal union, and the reason is that the spec
 * publishes no closed list for this plane: a code the panel has never seen still reaches the screen
 * with the gateway's own message beside it.
 */
export const schemaDataPlaneErrorDetail = z.object({
	code: z.string(),
	type: z.string(),
	message: z.string()
});

export const schemaDataPlaneError = z.object({
	error: schemaDataPlaneErrorDetail
});

export type DataPlaneError = z.infer<typeof schemaDataPlaneError>;
export type DataPlaneErrorDetail = z.infer<typeof schemaDataPlaneErrorDetail>;

/**
 * The panel's own failure vocabulary for this screen.
 *
 * The panel's route is a plane of its own, so it speaks its own envelope rather than forwarding the
 * gateway's: two vocabularies share the name `UNAUTHORIZED`, and a browser that could not tell them
 * apart would sign the operator out for a bad gateway key. Each code names whose failure it is, which
 * is what the screen needs to say something true.
 */
export const PLAYGROUND_ERROR_CODES = [
	/** The panel has no gateway key configured, so there is nothing to send with. */
	'PLAYGROUND_KEY_MISSING',
	/** The caller's panel session is not valid. The browser signs out on this one. */
	'UNAUTHORIZED',
	/** The browser sent a body the panel's own schema refuses. */
	'VALIDATION_ERROR',
	/** The panel could not dial the gateway at all. */
	'GATEWAY_UNREACHABLE',
	/** The gateway refused the panel's key. A key problem, never a session problem. */
	'GATEWAY_KEY_REFUSED',
	/** The gateway answered a failure of its own: an unknown model, an unhealthy upstream, a timeout. */
	'GATEWAY_ERROR',
	/** The panel's own configuration is wrong, and the message names the variable. */
	'INTERNAL_ERROR'
] as const;

export const playgroundErrorCode = z.enum(PLAYGROUND_ERROR_CODES);
export type PlaygroundErrorCode = z.infer<typeof playgroundErrorCode>;

/**
 * The panel's failure envelope.
 *
 * `gateway` carries the gateway's own error object when there was one, so the screen can show the
 * machine code a developer is looking for without inventing a second envelope for it.
 */
export const schemaPlaygroundFailure = z.object({
	error: z.object({
		code: playgroundErrorCode,
		message: z.string(),
		gateway: schemaDataPlaneErrorDetail.optional()
	})
});

export type PlaygroundFailure = z.infer<typeof schemaPlaygroundFailure>;

// One English line per data-plane code, so an empty message from the gateway still leaves the operator
// with something to act on. The list mirrors the gateway's own vocabulary (app-serv
// internal/dataplane/errors.go); a code outside it falls through to the last sentence rather than
// being reported as a contract break, because this plane's set is open by design.
const DATA_PLANE_FALLBACK: Record<string, string> = {
	VALIDATION_ERROR: 'The gateway refused the request body.',
	UNAUTHORIZED: 'The gateway refused the key the panel sent.',
	RATE_LIMITED: 'The gateway is rate limiting this key. Try again in a moment.',
	INTERNAL_ERROR: 'The gateway hit an unexpected error.',
	NO_PROVIDER_AVAILABLE: 'No upstream endpoint is healthy for that model right now.',
	UPSTREAM_ERROR: 'The upstream provider returned an error.',
	UPSTREAM_TIMEOUT: 'The upstream provider did not answer in time.',
	MODEL_NOT_FOUND: 'The gateway routes no model by that string.',
	PROVIDER_NOT_ROUTABLE: 'The gateway has no translator for that provider format.'
};

const UNKNOWN_CODE_SENTENCE =
	'The gateway refused the request with a code this panel does not know.';

/** The sentence to render beside a data-plane failure: the gateway's own message first, a fallback second. */
export function dataPlaneErrorSentence(failure: DataPlaneErrorDetail): string {
	const message = failure.message.trim();
	if (message.length > 0) return message;
	return DATA_PLANE_FALLBACK[failure.code] ?? UNKNOWN_CODE_SENTENCE;
}

// One English line per panel code, so a failure with an empty message still tells the operator what to
// do. The server writes a message of its own; this is the floor under it, and the browser's own copy
// for a state it can name but did not cause.
const PLAYGROUND_FALLBACK: Record<PlaygroundErrorCode, string> = {
	PLAYGROUND_KEY_MISSING: 'The panel has no gateway key configured.',
	UNAUTHORIZED: 'Your session ended. Sign in again to continue.',
	VALIDATION_ERROR: 'The panel refused the request before sending it.',
	GATEWAY_UNREACHABLE: 'The panel could not reach the gateway.',
	GATEWAY_KEY_REFUSED: 'The gateway refused the key the panel sent.',
	GATEWAY_ERROR: 'The gateway could not complete the request.',
	INTERNAL_ERROR: 'The panel is misconfigured.'
};

/** The sentence for a panel failure, preferring whatever the server said. */
export function playgroundFailureSentence(failure: PlaygroundFailure): string {
	const message = failure.error.message.trim();
	return message.length > 0 ? message : PLAYGROUND_FALLBACK[failure.error.code];
}
