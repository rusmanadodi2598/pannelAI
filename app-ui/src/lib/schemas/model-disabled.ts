// The disabled model set, mirroring docs/SPEC-API/001-SPEC-API.md §7.6 and docs/SPEC-UI/001-SPEC-UI.md
// §6.3.
//
// §6.3 asks for per-model enable and disable state. The API's shape decides what that means: the merged
// catalog *excludes* disabled rows (internal/service/model_catalog.go), so a disabled model has no row in
// the catalog and this set is the only place it is visible. The screen therefore renders both halves, and
// this module owns the set operations both halves share.
//
// Every operation below works on the WHOLE set, never on one provider's slice. `PUT /models/disabled` is
// an authoritative replace across all providers, so a write built from a slice would delete every other
// provider's rows. `providerDisabledRefs` narrows for display only and is never the input to a write.

import { z } from 'zod';

// One pair on the wire. Both fields are required by the DTO and validated by the domain: a ref with a
// blank segment is refused before it is stored. Tolerant of additive fields because it parses a response
// (SPEC-UI §7.4.2), and the panel builds the same shape for the write.
export const schemaDisabledRef = z.object({
	provider_id: z.string().min(1),
	model_id: z.string().min(1)
});

export type DisabledRef = z.infer<typeof schemaDisabledRef>;

// The GET and PUT bodies share one shape: the PUT answers the new set, so the panel reads the server's
// truth back instead of reconstructing it.
export const schemaDisabledSet = z.object({
	data: z.array(schemaDisabledRef)
});

export type DisabledSet = z.infer<typeof schemaDisabledSet>;

// The PUT body. `models` is required rather than optional so an empty body is a decode failure rather
// than a silent clear of the whole set.
export const schemaReplaceDisabledBody = z.strictObject({
	models: z.array(schemaDisabledRef)
});

export type ReplaceDisabledBody = z.infer<typeof schemaReplaceDisabledBody>;

// The key the API keys on and the server orders by.
export function disabledRefKey(ref: DisabledRef): string {
	return `${ref.provider_id}/${ref.model_id}`;
}

// Orders by provider then model, which is the repository's `ORDER BY provider_id, model_id`. The panel
// and the server therefore describe the same set in the same order, so a diff between them is empty.
function byRef(left: DisabledRef, right: DisabledRef): number {
	const leftKey = disabledRefKey(left);
	const rightKey = disabledRefKey(right);
	return leftKey < rightKey ? -1 : leftKey > rightKey ? 1 : 0;
}

// The set plus one ref. Applied to the whole last-read set: the ref is deduped by key, so calling this
// with a ref the set already holds returns the same set rather than a second copy.
export function withDisabledRef(refs: readonly DisabledRef[], ref: DisabledRef): DisabledRef[] {
	const merged = new Map<string, DisabledRef>();
	for (const entry of refs) merged.set(disabledRefKey(entry), entry);
	merged.set(disabledRefKey(ref), { provider_id: ref.provider_id, model_id: ref.model_id });
	return [...merged.values()].sort(byRef);
}

// The set minus one ref. Removing a ref the set does not hold is a no-op rather than an error: the caller
// asked for a state, and that state already holds.
export function withoutDisabledRef(refs: readonly DisabledRef[], ref: DisabledRef): DisabledRef[] {
	const key = disabledRefKey(ref);
	return refs.filter((entry) => disabledRefKey(entry) !== key).sort(byRef);
}

// One provider's slice, for display. Never feed this back into a write.
export function providerDisabledRefs(
	refs: readonly DisabledRef[],
	providerId: string
): DisabledRef[] {
	return refs.filter((ref) => ref.provider_id === providerId).sort(byRef);
}
