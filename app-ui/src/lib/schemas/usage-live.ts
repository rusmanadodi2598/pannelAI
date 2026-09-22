// The Usage live stream's frame (docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md §10.3).
//
// The route this reads, `GET /api/v1/usage/live`, is not served yet (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md
// F4), so this file is the contract the panel builds against rather than a mirror of a route that exists.
// Two consequences shape it, and both are about surviving a gateway the panel does not control:
//
//   Every collection is `nullableList`, because Go marshals a nil slice as `null` rather than as `[]`.
//
//   Every field the screen does not draw is optional. A frame that carries a field this panel never reads
//   is still a frame, and rejecting it would turn a live view into an error state over a detail that does
//   not matter. Unknown keys are stripped, which is what "tolerated" means here.
//
// The frame is full state for the three fields it carries: the gateway sends the whole in-flight set, not
// a delta. `usage-live-view.ts` is where that becomes the screen's state.

import { z } from 'zod';
import { nullableList, rfc3339Timestamp, schemaRequestStatus } from './primitives';

// One request the gateway is routing right now.
//
// `started_at` is required rather than optional, and it is the only required field besides the provider:
// it is the sole input to the panel's staleness guard. Without it the panel cannot tell a request that is
// still running from one whose marker the gateway never cleared, and it would light a node forever.
export const schemaUsageLiveActive = z.object({
	provider_id: z.string().min(1),
	endpoint_id: z.string().optional(),
	model: z.string().optional(),
	started_at: rfc3339Timestamp
});

export type UsageLiveActive = z.infer<typeof schemaUsageLiveActive>;

// One request that finished recently. The gateway bounds this list (draft 010 §10.3 asks for the same
// twenty the reference keeps), so the screen renders it rather than paging it.
export const schemaUsageLiveRecent = z.object({
	request_id: z.string().min(1),
	provider_id: z.string().min(1),
	model: z.string().optional(),
	ts: rfc3339Timestamp.optional(),
	status: schemaRequestStatus.optional(),
	tokens_in: z.number().int().min(0).optional(),
	tokens_out: z.number().int().min(0).optional(),
	error_code: z.string().optional()
});

export type UsageLiveRecent = z.infer<typeof schemaUsageLiveRecent>;

export const schemaUsageLiveFrame = z.object({
	active: nullableList(schemaUsageLiveActive).optional(),
	recent: nullableList(schemaUsageLiveRecent).optional(),
	// The provider the gateway last reported an error for, or an empty string. Optional so that a frame
	// which omits it leaves the previous statement standing rather than silently clearing it.
	error_provider: z.string().optional()
});

export type UsageLiveFrame = z.infer<typeof schemaUsageLiveFrame>;
