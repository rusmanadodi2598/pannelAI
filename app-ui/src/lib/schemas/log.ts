// Request log schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.13.
//
// Three wire facts shape this file. The list row carries `has_bodies` rather than the bodies themselves,
// so a page of a hundred requests does not carry a hundred payloads. The detail carries the capture
// setting beside the bodies, because "capture is off" and "capture is on with nothing stored" are
// different answers and the operator needs the difference. And the console read carries `max_records`,
// which is the bound in force, so the screen can say why older lines are gone instead of implying the
// buffer always held that much.
//
// The bodies render verbatim in an escaped, read-only block (SPEC-UI §7.5.1). Nothing here rewrites them:
// a stored payload is evidence, and normalizing it would make the audit view lie about what the gateway
// recorded. `capture_body_max_bytes` is carried so the screen can say why a body stops where it does.

import { z } from 'zod';
import {
	nullableList,
	pageMeta,
	rfc3339Timestamp,
	schemaRequestStatus,
	stringList
} from './primitives';

export const schemaLogRecord = z.object({
	request_id: z.string().min(1),
	ts: rfc3339Timestamp,
	gateway_key_id: z.string().optional(),
	endpoint_id: z.string().optional(),
	provider_id: z.string().optional(),
	model: z.string().optional(),
	status: schemaRequestStatus,
	latency_ms: z.number().int().min(0),
	error: z.string().optional(),
	has_bodies: z.boolean()
});

export type LogRecord = z.infer<typeof schemaLogRecord>;

export const schemaLogList = z.object({
	data: nullableList(schemaLogRecord),
	meta: pageMeta
});

export type LogList = z.infer<typeof schemaLogList>;

// The captured request and response of one request. Identical to the shape the usage detail joins, so the
// two screens cannot disagree about what a stored body looks like.
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

// The purge reports how many rows it removed. The number is real, so the confirmation can say what
// happened rather than only that something did.
export const schemaLogPurge = z.object({
	deleted: z.number().int().min(0)
});

export type LogPurge = z.infer<typeof schemaLogPurge>;

// The console ring buffer. `lines` is oldest first, which is the order the API sends and the order the
// screen appends, so nothing reverses it. `max_records` is the ceiling that evicted the older lines.
export const schemaConsoleLog = z.object({
	lines: stringList,
	max_records: z.number().int().min(0)
});

export type ConsoleLog = z.infer<typeof schemaConsoleLog>;
