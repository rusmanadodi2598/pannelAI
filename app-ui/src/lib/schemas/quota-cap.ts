// The budget-cap schemas, the form's rules, and the mappings between them (docs/SPEC-UI/001-SPEC-UI.md
// §6.6, U2; docs/SPEC-API/001-SPEC-API.md §7.12).
//
// Split from `quota.ts` so the window read contracts and the cap write shape stay separate files, the way
// the combo read and the combo editor are: a reader looking for "what does the API return for a window"
// should not have to walk past a form's cross-field rules.
//
// A cap belongs to an endpoint rather than to a window, and it is the ceiling the selector reads to skip
// an endpoint once the month's spend reaches it. Two shapes follow from that: the read keeps `null` for a
// cap that was never stored, because "no cap" and "a cap of zero" are opposite rules, and the write spells
// "clear this cap" by omitting the key, because the route replaces the whole cap set. The bounds and the
// degenerate-zero rule are the API's own (`domain.ValidateQuotaCapValues`), restated here so the form
// refuses a bad cap before the round trip instead of learning the rule from a 400.

import { z } from 'zod';
import { costString, nullableList, optionalTimestamp } from './primitives';
import { schemaQuotaWindow } from './quota';
import { formatCount } from './usage-view';

/** The largest monthly cost cap the API accepts, mirrored so the form refuses a larger one locally. */
export const MAX_QUOTA_MONTHLY_COST_USD = 1_000_000_000;

/** The largest monthly token cap the API accepts, mirrored for the same reason. */
export const MAX_QUOTA_MONTHLY_TOKENS = 1_000_000_000_000;

// A stored cap. `monthly_cost_usd` is the API's 8-place decimal string, never a number (SPEC-API §4), and
// both amounts are `omitempty` on the wire, so an absent field and an explicit null both mean "no cap is
// set". The two are kept as they arrive rather than normalized: `quotaCapForm` and `quotaCapSummary` are
// the places that read "unset" out of either spelling.
export const schemaQuotaCap = z.object({
	endpoint_id: z.string().min(1),
	monthly_cost_usd: costString.nullish(),
	monthly_tokens: z.number().int().min(0).nullish(),
	updated_at: optionalTimestamp
});

export type QuotaCap = z.infer<typeof schemaQuotaCap>;

// The per-endpoint read. `cap` carries no `omitempty` on the Go side, so an endpoint with nothing stored
// answers an explicit null, which the form renders as two empty fields rather than as a cap of zero.
export const schemaQuotaEndpointDetail = z.object({
	endpoint_id: z.string().min(1),
	cap: schemaQuotaCap.nullable(),
	data: nullableList(schemaQuotaWindow)
});

export type QuotaEndpointDetail = z.infer<typeof schemaQuotaEndpointDetail>;

/** The cap form's two fields, as the operator types them. A blank field means "clear this cap". */
export type QuotaCapForm = { cost: string; tokens: string };

/** The request body `PUT /quotas/{endpoint_id}` takes. An omitted amount clears that cap. */
export type QuotaCapBody = { monthly_cost_usd?: string; monthly_tokens?: number };

// The form's rules, plus the one cross-field rule the API enforces: a zero cost cap that stands alone is
// refused there because the router would skip the endpoint after the first fraction of a cent, which is
// never what a caller meant.
export const schemaQuotaCapForm = z
	.object({
		cost: z
			.string()
			.transform((value) => value.trim())
			.refine((value) => value === '' || /^\d+(\.\d+)?$/.test(value), {
				message: 'Write the monthly cost as a plain amount in USD, for example 25 or 25.50.'
			})
			.refine((value) => value === '' || Number(value) <= MAX_QUOTA_MONTHLY_COST_USD, {
				message: `A monthly cost cannot exceed ${formatCount(MAX_QUOTA_MONTHLY_COST_USD)} USD.`
			}),
		tokens: z
			.string()
			.transform((value) => value.trim())
			.refine((value) => value === '' || /^\d+$/.test(value), {
				message: 'Write the monthly tokens as a whole number, for example 1000000.'
			})
			.refine((value) => value === '' || Number(value) <= MAX_QUOTA_MONTHLY_TOKENS, {
				message: `Monthly tokens cannot exceed ${formatCount(MAX_QUOTA_MONTHLY_TOKENS)}.`
			})
	})
	.superRefine((form, ctx) => {
		if (form.cost !== '' && Number(form.cost) === 0 && form.tokens === '') {
			ctx.addIssue({
				code: 'custom',
				path: ['cost'],
				message:
					'A cost cap of zero would stop the router picking this endpoint, so set a token cap beside it or leave the cost empty.'
			});
		}
	});

/** The form as the request body. A blank field is omitted, which is how the wire clears that cap. */
export function buildQuotaCapBody(form: QuotaCapForm): QuotaCapBody {
	const body: QuotaCapBody = {};
	const cost = form.cost.trim();
	const tokens = form.tokens.trim();

	if (cost !== '') body.monthly_cost_usd = cost;
	if (tokens !== '') body.monthly_tokens = Number(tokens);
	return body;
}

/** A stored cap as the form's fields, or two empty fields when nothing is stored. */
export function quotaCapForm(cap: QuotaCap | null): QuotaCapForm {
	const tokens = cap?.monthly_tokens ?? null;
	return {
		cost: trimDecimal(cap?.monthly_cost_usd ?? ''),
		tokens: tokens === null ? '' : String(tokens)
	};
}

/**
 * What the stored cap means for routing, as the operator reads it.
 *
 * The empty case is a rule rather than an absence: no cap means the selector never skips the endpoint for
 * budget, so the sentence says so instead of printing a ceiling of zero.
 */
export function quotaCapSummary(cap: QuotaCap | null): string {
	const cost = cap?.monthly_cost_usd ?? null;
	const tokens = cap?.monthly_tokens ?? null;
	if (cost === null && tokens === null) {
		return 'No cap is stored for this endpoint, so the router picks it whenever it is healthy.';
	}

	const parts: string[] = [];
	if (cost !== null) parts.push(`${trimDecimal(cost)} USD`);
	if (tokens !== null) parts.push(`${formatCount(tokens)} tokens`);
	return `This endpoint is capped at ${parts.join(' and ')} a month.`;
}

/**
 * The consequence §6.6 asks the screen to state where a cap is saved.
 *
 * A cap is not a report: the selector reads it and skips the endpoint, so a save takes the endpoint out of
 * rotation the moment the month-to-date spend reaches it, and a request that needed it fails over or
 * fails.
 */
export const QUOTA_CAP_WARNING =
	'Saving a cap changes routing: once the month-to-date spend reaches it, the router stops picking this endpoint.';

/**
 * The endpoints the cap form offers: every endpoint the label read returned, then any endpoint the window
 * table names that the read did not cover.
 *
 * The window table can name an endpoint past the list's first page, and a cap belongs to an endpoint the
 * gateway knows, so both sources are offered rather than the label read alone. Sorted by what the
 * operator reads, which is the label.
 */
export function quotaCapOptions(
	labels: Map<string, string>,
	windowEndpointIds: string[]
): { id: string; label: string }[] {
	const options = new Map<string, string>(labels);
	for (const id of windowEndpointIds) {
		if (!options.has(id)) options.set(id, id);
	}

	return [...options]
		.map(([id, label]) => ({ id, label }))
		.sort(
			(left, right) => left.label.localeCompare(right.label) || left.id.localeCompare(right.id)
		);
}

// A stored amount prints with all 8 decimal places, so a cap of 25 reads back as `25.00000000`. The form
// and the sentence print the amount without the trailing zeros, and a value that carries real precision
// keeps it.
function trimDecimal(value: string): string {
	if (!value.includes('.')) return value;
	return value.replace(/\.?0+$/, '');
}
