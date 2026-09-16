// Gateway key schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.3 and the table in §6.
//
// `status` is a plain string on purpose: SPEC-API §7.3 accepts `status` on PATCH but does not
// enumerate its values, and inventing an enum here would make the panel reject a value the API
// accepts. The panel renders unknown values verbatim. See SPEC-UI §14 Q9.

import { z } from 'zod';
import { gatewayKeyId, label, nullableTimestamp, rfc3339Timestamp, tokenCount } from './primitives';

export const schemaGatewayKey = z.object({
	id: gatewayKeyId,
	name: label,
	key_hint: z.string(),
	status: z.string().min(1),
	last_used_at: nullableTimestamp,
	request_count: tokenCount,
	created_at: rfc3339Timestamp,
	revoked_at: nullableTimestamp
});

export type GatewayKey = z.infer<typeof schemaGatewayKey>;

export const schemaGatewayKeyList = z.object({
	data: z.array(schemaGatewayKey),
	meta: z.object({
		page: z.number().int(),
		per_page: z.number().int(),
		total: z.number().int()
	})
});

export type GatewayKeyList = z.infer<typeof schemaGatewayKeyList>;

export const schemaCreateGatewayKeyForm = z.strictObject({
	name: label
});

export type CreateGatewayKeyForm = z.infer<typeof schemaCreateGatewayKeyForm>;

export const schemaUpdateGatewayKeyForm = z.strictObject({
	name: label.optional(),
	status: z.string().min(1).optional()
});

export type UpdateGatewayKeyForm = z.infer<typeof schemaUpdateGatewayKeyForm>;

// The plaintext key is returned exactly once, on create. The panel shows it in a modal and never
// stores it (SPEC-UI §4, §8.5).
export const schemaCreatedGatewayKey = z.object({
	id: gatewayKeyId,
	name: label,
	key: z.string().min(1),
	key_hint: z.string(),
	created_at: rfc3339Timestamp
});

export type CreatedGatewayKey = z.infer<typeof schemaCreatedGatewayKey>;

// Values the panel writes when toggling a key. Listed here rather than inline so a change is one
// edit, and marked pending confirmation in SPEC-UI §14 Q9.
export const KEY_STATUS_ACTIVE = 'active';
export const KEY_STATUS_DISABLED = 'disabled';
