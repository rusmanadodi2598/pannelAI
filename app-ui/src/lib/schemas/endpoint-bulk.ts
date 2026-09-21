// The batch wire shapes for adding several keys at once (docs/SPEC-API/001-SPEC-API.md §8.1, SPEC-UI §6.2).
//
// A batch is all-or-nothing: either every row is stored or none is, and the answer says which. That makes
// two shapes, not one. `schemaBulkKeyResult` is the accepted batch, carrying the created keys. A refused
// batch answers with an envelope naming the failure plus `results`, one entry per row by index, with the
// message on the offending one; `schemaBulkKeyRefusal` is that shape, and it is parsed rather than dropped
// because a screen shown only the envelope would leave the operator to guess which row to fix.
//
// These are read shapes for one route, kept apart from the endpoint read shapes and the write forms in
// `./endpoint` so neither module grows past the panel's line limit.

import { z } from 'zod';
import { schemaEndpointKey } from './endpoint';

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

/** A refused batch: the envelope names the failure, `results` names the row, and nothing was stored. */
export const schemaBulkKeyRefusal = z.object({
	error: z.object({ code: z.string(), message: z.string() }),
	results: z.array(schemaBulkResultRow)
});

export type BulkKeyRefusal = z.infer<typeof schemaBulkKeyRefusal>;
