// Token saver contract, mirroring docs/SPEC-API/001-SPEC-API.md §7.9 and the sections in
// docs/SPEC-UI/001-SPEC-UI.md §6.7.
//
// Three groups ship: RTK, Headroom, and Ponytail. `caveman` is DEPRECATED (owner, 2026-09-16): the API
// keeps the key frozen for round-trip compatibility and §6.7 forbids the panel rendering a section, a
// control, a label, or an upgrade hint for it. The delivered response does not carry the key at all, so
// nothing here parses it and nothing can render it.
//
// The read and the write carry the same document, which is what lets the panel PUT back exactly what it
// GET. The write is a whole-document replacement, so a save is a read-modify-write: the draft helpers in
// `token-saver-form.ts` merge one group over the last document that was read, which is how saving RTK
// cannot rewrite Headroom.
//
// `rtk.filters` is an allowlist over the twelve canonical names (SPEC-API-002 §4.1), and an empty list is
// a valid configuration meaning every filter is eligible (§4.2). The panel keeps that distinction in the
// copy, because "no filter selected" and "every filter eligible" are the same stored value and only one
// of them is true.

import { z } from 'zod';
import { optionalAbsoluteUrl } from './primitives';

// The twelve canonical filter names and what each one compacts, taken from the cap table in
// SPEC-API-002 §5.1 so the screen describes the engine that runs rather than a paraphrase of it.
export const RTK_FILTERS: { name: string; description: string }[] = [
	{ name: 'git-diff', description: 'Compacted diff: 500 lines total, 100 per hunk.' },
	{ name: 'git-status', description: 'Compacted status: 10 files per group, 10 untracked.' },
	{ name: 'git-log', description: 'Compacted log: 200 lines.' },
	{ name: 'grep', description: 'Grep output: 10 matches per file.' },
	{ name: 'find', description: 'Find output: 10 files per directory, 20 directories.' },
	{ name: 'ls', description: 'Directory listing: 5 extensions in the summary.' },
	{ name: 'tree', description: 'Tree listing: 200 lines.' },
	{ name: 'dedup-log', description: 'Deduplicated log: 2000 lines.' },
	{ name: 'smart-truncate', description: 'Generic fallback above 250 lines: 120 head, 60 tail.' },
	{ name: 'read-numbered', description: 'Numbered file dumps, from 250 lines.' },
	{ name: 'search-list', description: 'Search results: 10 files per directory, 20 directories.' },
	{ name: 'build-output', description: 'Build logs: 5 warnings kept, 3 deprecation notices.' }
];

export const RTK_FILTER_NAMES: string[] = RTK_FILTERS.map((filter) => filter.name);

// The levels the API accepts (`oneof=lite full ultra`, §7.9). A closed set, so a value outside it is a
// parse failure the operator can report rather than a control that renders something meaningless.
export const TOKEN_SAVER_LEVELS = ['lite', 'full', 'ultra'] as const;
export type TokenSaverLevel = (typeof TOKEN_SAVER_LEVELS)[number];

export const TOKEN_SAVER_LEVEL_LABELS: Record<TokenSaverLevel, string> = {
	lite: 'Lite',
	full: 'Full',
	ultra: 'Ultra'
};

export const TOKEN_SAVER_LEVEL_EXPLANATIONS: Record<TokenSaverLevel, string> = {
	lite: 'The lightest bias: the least rewriting, the smallest saving.',
	full: 'The default bias: the engine rewrites a blob when a filter claims it.',
	ultra: 'The strongest bias: the engine rewrites whenever it can.'
};

// One saver group with a level.
const schemaLevelGroup = z.object({
	enabled: z.boolean(),
	level: z.enum(TOKEN_SAVER_LEVELS)
});

// The RTK group as the API returns it. `filters` is an array of plain strings rather than an enum: the
// panel must parse a name it does not know so it can preserve it, and an enum would turn a stored
// configuration into a drift failure the operator cannot act on.
const schemaRtkGroup = z.object({
	enabled: z.boolean(),
	filters: z.array(z.string().min(1))
});

// The external compression group as the API returns it. The URL is optional because the group is usually
// disabled; the write schema below is the one that checks its shape.
const schemaHeadroomGroup = z.object({
	enabled: z.boolean(),
	url: z.string(),
	compress_user_messages: z.boolean()
});

export const schemaTokenSaver = z.object({
	rtk: schemaRtkGroup,
	headroom: schemaHeadroomGroup,
	ponytail: schemaLevelGroup
});

export type TokenSaver = z.infer<typeof schemaTokenSaver>;

// One group as the editor validates it. These are the schemas a section save parses, so an invalid value
// in another section is not a reason to refuse this one.
export const schemaRtkSection = z.object({
	enabled: z.boolean(),
	filters: z.array(z.string().min(1).max(64))
});

export const schemaHeadroomSection = z.object({
	enabled: z.boolean(),
	url: optionalAbsoluteUrl,
	compress_user_messages: z.boolean()
});

export const schemaPonytailSection = z.object({
	enabled: z.boolean(),
	level: z.enum(TOKEN_SAVER_LEVELS)
});

// The replacement body PUT /api/v1/token-saver accepts: the whole document, every group required. Strict,
// so a field the panel invents fails here instead of being ignored by the API.
export const schemaTokenSaverBody = z.strictObject({
	rtk: schemaRtkSection,
	headroom: schemaHeadroomSection,
	ponytail: schemaPonytailSection
});

export type TokenSaverBody = z.infer<typeof schemaTokenSaverBody>;

export type TokenSaverSection = 'rtk' | 'headroom' | 'ponytail';

export const TOKEN_SAVER_SECTION_LABELS: Record<TokenSaverSection, string> = {
	rtk: 'RTK',
	headroom: 'Headroom',
	ponytail: 'Ponytail'
};

export const TOKEN_SAVER_SECTION_SCHEMAS = {
	rtk: schemaRtkSection,
	headroom: schemaHeadroomSection,
	ponytail: schemaPonytailSection
} as const;

/**
 * The whole replacement body, built from the current draft.
 *
 * Every group is written on every save, because the API replaces the document. The two groups the
 * operator did not touch are written back exactly as they were read, which is what "saving one section
 * never rewrites another" means against a whole-document route. The allowlist is copied, so a later draft
 * edit cannot change a body that was already built.
 */
export function buildTokenSaverBody(form: TokenSaver): TokenSaverBody {
	return {
		rtk: { enabled: form.rtk.enabled, filters: [...form.rtk.filters] },
		headroom: {
			enabled: form.headroom.enabled,
			url: form.headroom.url,
			compress_user_messages: form.headroom.compress_user_messages
		},
		ponytail: { enabled: form.ponytail.enabled, level: form.ponytail.level }
	};
}
