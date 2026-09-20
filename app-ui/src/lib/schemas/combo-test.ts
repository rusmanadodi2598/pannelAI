// The combo test answer, from docs/SPEC-API/001-SPEC-API.md §7.7 and docs/SPEC-UI/001-SPEC-UI.md §6.4.
//
// The route probes every reference a combo stores, one at a time, and reports each one's own outcome. A
// reference that failed is a RESULT rather than a route error, because the point of the click is to learn
// which member is down while the members that answered are still reported. So the schema's job is to keep a
// failure readable without inventing anything: `ok`, the latency, and, when the probe failed, the data
// plane's own code and message. The identity fields are `omitempty` on the wire, so a probe that failed
// before it resolved a target carries none of them.
//
// `role` and `strategy` are read as loose strings. §7.7 names `model` and `judge` today; a third role would
// render as its own word instead of failing the read.

import { z } from 'zod';

const ROLE_MODEL = 'model';
const ROLE_JUDGE = 'judge';

export const schemaComboProbeResult = z.object({
	ref: z.string().min(1),
	role: z.string(),
	ok: z.boolean(),
	provider_id: z
		.string()
		.nullish()
		.transform((value) => value ?? ''),
	model_id: z
		.string()
		.nullish()
		.transform((value) => value ?? ''),
	endpoint_id: z
		.string()
		.nullish()
		.transform((value) => value ?? ''),
	latency_ms: z.number().int().min(0),
	error_code: z
		.string()
		.nullish()
		.transform((value) => value ?? ''),
	error: z
		.string()
		.nullish()
		.transform((value) => value ?? '')
});

export type ComboProbeResult = z.infer<typeof schemaComboProbeResult>;

export const schemaComboTest = z.object({
	combo_id: z.string().min(1),
	combo: z.string().min(1),
	strategy: z.string(),
	results: z
		.array(schemaComboProbeResult)
		.nullish()
		.transform((value) => value ?? [])
});

export type ComboTest = z.infer<typeof schemaComboTest>;

/** The role as the result list labels it. A role the panel does not know renders as its own word. */
export function comboProbeRoleLabel(role: string): string {
	if (role === ROLE_JUDGE) return 'Judge';
	if (role === ROLE_MODEL) return 'Model';
	return role;
}

/**
 * The one-line count of what answered. A zero-reference answer is the API's own shape for a combo that
 * stores nothing, which it refuses to save but still reports, so it is said rather than rendered as an
 * empty list.
 */
export function comboProbeSummary(results: readonly ComboProbeResult[]): string {
	const total = results.length;
	if (total === 0) return 'This combo stores no references, so there was nothing to probe.';

	const answered = results.filter((result) => result.ok).length;
	if (answered === total) {
		return total === 1 ? 'The only reference answered.' : `All ${total} references answered.`;
	}
	if (answered === 0) {
		return total === 1
			? 'The only reference did not answer.'
			: `None of the ${total} references answered.`;
	}
	return `${answered} of ${total} references answered.`;
}

/**
 * What a probe resolved to. A reference may be an alias or a nested combo, so the stored ref and the
 * resolved identity are two different facts and this is the second one. A failed probe that never reached a
 * provider has neither part, and the panel says so instead of leaving a blank that reads as a defect.
 */
export function comboProbeIdentity(result: ComboProbeResult): string {
	const parts = [result.provider_id, result.model_id].filter((part) => part !== '');
	if (parts.length === 0) return 'No identity was reported.';
	return parts.join('/');
}
