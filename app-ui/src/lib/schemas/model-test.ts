// The provider model test answer, from docs/SPEC-API/001-SPEC-API.md §7.4 and draft 017 §4.10.
//
// The route asks a question the connection test cannot answer: not "does this credential reach the
// provider" but "does this model answer". A model that failed is a ROW, not a route error, so the schema
// keeps a failure readable without inventing a cause the API did not name.
//
// `status` reads as 0 when absent. The wire marks it `omitempty` because a healthy probe never saw a code
// the gateway could report, and defaulting it to 200 would put a figure on screen that no response carried
// (R-17). `source` and `warning` are the model list's own origin, carried for the same reason the catalog
// read carries them: a sweep over a stale list proves less than a sweep over the live one.

import { z } from 'zod';

/** The code the server names when a probe ran out of its budget rather than being refused. */
export const MODEL_TEST_TIMEOUT_CODE = 'MODEL_TEST_TIMEOUT';

/**
 * The number of models the sweep probes when the operator asks for the default. It mirrors the server's
 * own default, and the button says it, because every row is a real inference call against the provider
 * account's quota (R-36: a control states what it costs).
 */
export const MODEL_TEST_SWEEP_LIMIT = 6;

const text = z
	.string()
	.nullish()
	.transform((value) => value ?? '');

export const schemaModelTestResult = z.object({
	model_id: z.string().min(1),
	name: text,
	ok: z.boolean(),
	latency_ms: z
		.number()
		.int()
		.min(0)
		.nullish()
		.transform((value) => value ?? 0),
	endpoint_id: text,
	status: z
		.number()
		.int()
		.nullish()
		.transform((value) => value ?? 0),
	error_code: text,
	error: text
});

export type ModelTestResult = z.infer<typeof schemaModelTestResult>;

export const schemaModelTestSweep = z.object({
	provider_id: z.string().min(1),
	source: z.string(),
	warning: text,
	tested: z.number().int().min(0),
	total: z.number().int().min(0),
	stopped: text,
	results: z
		.array(schemaModelTestResult)
		.nullish()
		.transform((value) => value ?? [])
});

export type ModelTestSweep = z.infer<typeof schemaModelTestSweep>;

/** The row a result belongs to, addressed the way a client addresses a model. */
export function modelTestRowKey(providerId: string, modelId: string): string {
	return `${providerId}/${modelId}`;
}

/**
 * Whether a chat probe can reach a model. This is the server's rule (`registry.Model.IsChat`) read from
 * the fields the catalog already returns: a model with no declared kind is chat, a media kind is not.
 *
 * It decides whether the panel offers the button at all. A control that can only produce a misleading
 * failure is a dead control (R-26), and an embedding model's refusal would describe the probe, not the
 * model.
 */
export function isChatRoutable(kind?: string): boolean {
	return kind === undefined || kind === '' || kind === 'llm' || kind === 'chat';
}

/** The row's own state line. A probe that never completed reports no latency rather than a zero. */
export function modelTestLine(result: ModelTestResult): string {
	if (result.latency_ms > 0) {
		return `${result.ok ? 'Answered' : 'Failed'} in ${result.latency_ms} ms`;
	}
	return result.ok ? 'Answered' : 'Failed';
}

/**
 * What a sweep covered, in one sentence a reader can act on. `tested` below `total` is said explicitly,
 * because a count of models that were never probed is the difference between "these answered" and "this
 * provider is healthy".
 */
export function modelTestSweepSummary(sweep: ModelTestSweep): string {
	if (sweep.tested === 0 && sweep.total === 0) {
		return 'This provider offers no chat model to test.';
	}

	const answered = sweep.results.filter((row) => row.ok).length;
	const failed = sweep.results.length - answered;
	const scope =
		sweep.tested === sweep.total
			? `Tested ${plural(sweep.tested, 'model')}`
			: `Tested ${sweep.tested} of ${sweep.total} models`;

	const tally =
		failed === 0
			? 'all answered'
			: answered === 0
				? 'none answered'
				: `${answered} answered, ${failed} failed`;

	const stopped =
		sweep.stopped === 'deadline' ? ' The sweep ran out of time before every model.' : '';

	// The warning is the API's own sentence about where the list came from. It is appended rather than
	// paraphrased: the panel has no better claim on that fact than the route that read it.
	const warning = sweep.warning === '' ? '' : ` ${sweep.warning}`;

	return `${scope}: ${tally}.${stopped}${warning}`;
}

function plural(count: number, noun: string): string {
	return `${count} ${noun}${count === 1 ? '' : 's'}`;
}
