// Alias contracts for docs/SPEC-API/001-SPEC-API.md §7.6 and docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// An alias is a short name a client may send in place of a model string. The set is global: neither the
// read nor the write route takes a provider, and the alias itself is the table's primary key. That is why
// the helpers here merge over the WHOLE set: a write built from one screen's slice would delete every
// alias the slice did not carry.
//
// Both field rules mirror app-serv's domain rather than tightening it. The alias is
// `validateCatalogText` plus the slash refusal in `NewModelAlias`, and the target is `validateComboRef`
// (SPEC-API §7.7's three shapes, of which the panel's set only ever holds the first two: the service
// refuses a target that names another alias). A panel rule the gateway does not share would refuse a
// write the API would have accepted, which is the failure mode this file exists to avoid.

import { z } from 'zod';

// The DTO's bounds for one alias entry.
export const ALIAS_NAME_MAX = 120;
export const ALIAS_TARGET_MAX = 200;

const CONTROL_CHARS = /[\u0000-\u001f\u007f]/;

// A target without a slash is a combo name, and the API accepts the same character set for one.
const COMBO_NAME_PATTERN = /^[A-Za-z0-9._-]+$/;

// An alias name. Internal spaces are accepted because the API accepts them: only control characters and
// the slash are refused, and the slash is refused because `a/b` is a provider/model reference and the two
// would be indistinguishable.
export const aliasName = z
	.string()
	.transform((value) => value.trim())
	.refine((value) => value.length > 0, { message: 'An alias is required.' })
	.refine((value) => value.length <= ALIAS_NAME_MAX, {
		message: `Use ${ALIAS_NAME_MAX} characters or fewer.`
	})
	.refine((value) => !CONTROL_CHARS.test(value), {
		message: 'An alias cannot contain control characters.'
	})
	.refine((value) => !value.includes('/'), {
		message: 'An alias cannot contain a slash; a slash makes it a provider/model reference.'
	});

// A target: `provider/model`, or a combo name. Whitespace is refused anywhere in it, which is what keeps a
// mistyped model string from being stored as a name nothing can resolve.
export const aliasTarget = z
	.string()
	.transform((value) => value.trim())
	.refine((value) => value.length > 0, { message: 'A target is required.' })
	.refine((value) => value.length <= ALIAS_TARGET_MAX, {
		message: `Use ${ALIAS_TARGET_MAX} characters or fewer.`
	})
	.refine((value) => !/\s/.test(value), { message: 'A target cannot contain spaces.' })
	.refine((value) => value.includes('/') || COMBO_NAME_PATTERN.test(value), {
		message: 'A target is provider/model or a combo name.'
	})
	// A reference needs both halves. The split is at the first slash, so the model half may itself contain
	// one, which is the shape `ParseModelRef` reads.
	.refine(
		(value) => {
			const slash = value.indexOf('/');
			return slash < 0 || (slash > 0 && slash < value.length - 1);
		},
		{ message: 'A model reference needs both halves: provider/model.' }
	);

// One entry, as the form takes it and as the write body carries it: same two fields, same rules. Strict,
// so a field the API does not read cannot ride along unnoticed.
export const schemaAliasEntry = z.strictObject({ alias: aliasName, target: aliasTarget });

export type ModelAliasEntry = z.infer<typeof schemaAliasEntry>;

// One entry as the API answers it. Read tolerantly, because a stored row is rendered even when it would
// not pass the write rules above: the API decides what can be written back, and a row it refuses is
// reported by its own message rather than by a screen that silently dropped it.
export const schemaModelAlias = z.object({ alias: z.string(), target: z.string() });

export type ModelAlias = z.infer<typeof schemaModelAlias>;

// The read body. Not paginated: aliases are hand-curated, so the bound is what the operator wrote.
export const schemaAliasSet = z.object({ data: z.array(schemaModelAlias) });

export type AliasSet = z.infer<typeof schemaAliasSet>;

// The write body: the whole set, every entry checked by the rules above.
export const schemaReplaceAliasesBody = z.strictObject({
	aliases: z.array(schemaAliasEntry)
});

// The set's identity, which is the alias itself. Trimmed, because that is the form the API stores.
export function aliasKey(entry: { alias: string }): string {
	return entry.alias.trim();
}

// The whole set with one entry added or changed, both fields normalized to the form the API stores. An
// existing name keeps its place and gets the new target, because `alias` is the primary key: a second row
// with the same name is a database error, so merging is the only shape that can succeed, and it makes this
// one helper the add path and the edit path.
export function withAlias(
	entries: readonly ModelAlias[],
	entry: ModelAliasEntry
): ModelAliasEntry[] {
	const merged = new Map<string, ModelAliasEntry>();
	for (const row of entries)
		merged.set(aliasKey(row), { alias: aliasKey(row), target: row.target.trim() });
	merged.set(aliasKey(entry), { alias: aliasKey(entry), target: entry.target.trim() });
	return [...merged.values()];
}

// The whole set without one alias, both fields normalized like `withAlias`. Every other row is carried, so
// removing one alias cannot remove two.
export function withoutAlias(entries: readonly ModelAlias[], name: string): ModelAliasEntry[] {
	const key = name.trim();
	return entries
		.filter((row) => aliasKey(row) !== key)
		.map((row) => ({ alias: aliasKey(row), target: row.target.trim() }));
}

// The PUT body, sorted by alias. The write answer echoes the request order while a fresh read is
// `ORDER BY alias`, so sending the read order is what keeps the table from reshuffling after a reload.
export function aliasSetBody(entries: readonly ModelAliasEntry[]): { aliases: ModelAliasEntry[] } {
	const sorted = entries
		.map((entry) => ({ alias: aliasKey(entry), target: entry.target.trim() }))
		.sort((left, right) => (left.alias < right.alias ? -1 : left.alias > right.alias ? 1 : 0));
	return { aliases: sorted };
}
