// Gateway key schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.3 and the table in §6.
//
// `status` is a plain string on the read schema because the response carries a third value the write
// path cannot set: `revoked`, reached only through DELETE (SPEC-API §7.3). The panel classifies that
// one value to hide the row (a terminal key's controls would all answer 409, and the reference's own
// list drops a deleted key), and passes every other value through verbatim. The write schema narrows
// to the two members the spec names.

import { z } from 'zod';
import { gatewayKeyId } from './identifiers';
import { label, optionalTimestamp, rfc3339Timestamp, tokenCount } from './primitives';

// `last_used_at` and `revoked_at` are `omitempty` on the wire (Go omits a nil pointer), so a key that was
// never used and never revoked omits both. `optionalTimestamp` accepts an absent field and an explicit
// null; `nullableTimestamp` would demand the field be there and fail the parse for the common case.
export const schemaGatewayKey = z.object({
	id: gatewayKeyId,
	name: label,
	key_hint: z.string(),
	status: z.string().min(1),
	last_used_at: optionalTimestamp,
	request_count: tokenCount,
	created_at: rfc3339Timestamp,
	revoked_at: optionalTimestamp
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

// Values the panel writes when toggling a key, and the only members SPEC-API §7.3 accepts on PATCH
// (`revoked` is reached only through DELETE). Listed here rather than inline so a change is one edit.
export const KEY_STATUS_ACTIVE = 'active';
export const KEY_STATUS_DISABLED = 'disabled';
export const KEY_STATUSES = [KEY_STATUS_ACTIVE, KEY_STATUS_DISABLED] as const;
export type KeyStatus = (typeof KEY_STATUSES)[number];

// The terminal read-only status. It is deliberately not a member of `KEY_STATUSES`, because nothing in
// the panel may write it: only DELETE produces it (SPEC-API §7.3), and the list hides the rows that
// carry it so a revoked key leaves the screen.
export const KEY_STATUS_REVOKED = 'revoked';

export const schemaUpdateGatewayKeyForm = z.strictObject({
	name: label.optional(),
	status: z.enum(KEY_STATUSES).optional()
});

export type UpdateGatewayKeyForm = z.infer<typeof schemaUpdateGatewayKeyForm>;

// The plaintext key is returned exactly once, on create, and the wire names it `plaintext_key`
// (`app-serv/internal/schema/dto.go`, `omitempty` so it is absent on every other response). SPEC-API §7.3
// says "returns full key once" without naming the field, so the served name is the one to parse: reading
// `key` instead made every create fail the parse and no one-time key ever reached the modal.
export const schemaCreatedGatewayKey = z.object({
	id: gatewayKeyId,
	name: label,
	plaintext_key: z.string().min(1),
	key_hint: z.string(),
	created_at: rfc3339Timestamp
});

export type CreatedGatewayKey = z.infer<typeof schemaCreatedGatewayKey>;
