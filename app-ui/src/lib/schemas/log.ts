// Request log schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.13.
//
// Only the detail shape lives here so far. The usage record detail (§7.12) joins a captured log, so the
// panel needs this shape to render one; the log list, purge, and console buffer belong to the `/logs` and
// `/console-log` screens and are added with them rather than declared ahead of a caller.
//
// The bodies render verbatim in an escaped, read-only block (SPEC-UI §7.5.1). Nothing here rewrites them:
// a stored payload is evidence, and normalizing it would make the audit view lie about what the gateway
// recorded. `capture_body_max_bytes` is carried so the screen can say why a body stops where it does.

import { z } from 'zod';
import { rfc3339Timestamp, schemaRequestStatus } from './primitives';

export const schemaLogDetail = z.object({
	request_id: z.string().min(1),
	ts: rfc3339Timestamp,
	gateway_key_id: z.string().optional(),
	endpoint_id: z.string().optional(),
	provider_id: z.string().optional(),
	model: z.string().optional(),
	status: schemaRequestStatus,
	latency_ms: z.number().int().min(0),
	error: z.string().optional(),
	capture_enabled: z.boolean(),
	capture_body_max_bytes: z.number().int().min(0),
	request_body: z.string().optional(),
	response_body: z.string().optional()
});

export type LogDetail = z.infer<typeof schemaLogDetail>;
