// The batch wire shapes for adding several rows at once (docs/SPEC-API/001-SPEC-API.md §8.1, SPEC-UI §6.2).
//
// A batch is all-or-nothing: either every row is stored or none is, and the answer says which. That makes
// two shapes, not one. The accepted shape carries the created rows plus a verdict per row by index; a
// refusal answers with an envelope naming the failure plus `results`, one entry per row by index, with
// the message on the offending one. `schemaBulkRefusal` is that second shape, and it is parsed rather
// than dropped because a screen shown only the envelope would leave the operator to guess which row to
// fix. Two routes answer with it, several keys on one endpoint and several connections at one provider,
// so it is named for the rule rather than for one of them.
//
// These are read shapes for those routes, kept apart from the endpoint read shapes and the write forms in
// `./endpoint` so neither module grows past the panel's line limit.

import { z } from 'zod';
import { schemaEndpoint, schemaEndpointKey } from './endpoint';

/** One batch row's verdict. §8.1 reports every row by index, so a refused batch still names the row. */
export const schemaBulkResultRow = z.object({
	index: z.number().int(),
	id: z.string().optional(),
	error: z.string().optional()
});

export type BulkResultRow = z.infer<typeof schemaBulkResultRow>;

/** The answer to POST /endpoints/{id}/keys/bulk: the stored keys plus the per-row verdicts. */
export const schemaBulkKeyResult = z.object({
	error: z.object({ code: z.string(), message: z.string() }).optional(),
	created: z.array(schemaEndpointKey),
	results: z.array(schemaBulkResultRow)
});

export type BulkKeyResult = z.infer<typeof schemaBulkKeyResult>;

/** The answer to POST /endpoints/bulk: the created connections, one per row of the batch (§7.5). */
export const schemaBulkEndpointResult = z.object({
	error: z.object({ code: z.string(), message: z.string() }).optional(),
	created: z.array(schemaEndpoint),
	results: z.array(schemaBulkResultRow)
});

export type BulkEndpointResult = z.infer<typeof schemaBulkEndpointResult>;

/** A refused batch: the envelope names the failure, `results` names the row, and nothing was stored. */
export const schemaBulkRefusal = z.object({
	error: z.object({ code: z.string(), message: z.string() }),
	results: z.array(schemaBulkResultRow)
});

export type BulkRefusal = z.infer<typeof schemaBulkRefusal>;
