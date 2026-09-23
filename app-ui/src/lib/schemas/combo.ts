// Combo schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.7 and the editor in
// docs/SPEC-UI/001-SPEC-UI.md §6.4.
//
// The strategy is the whole shape of a combo. It decides which of `sticky_limit` and `judge_model` means
// anything, and the API enforces that in the domain layer: `fallback` normalizes `sticky_limit` to 1 and
// refuses a `judge_model`, `round_robin` requires `sticky_limit` of at least 1 and refuses a `judge_model`,
// and `fusion` requires a `judge_model` and refuses a non-zero `sticky_limit`. The panel mirrors those
// rules so a save is refused before the round trip, and it hides the field a strategy ignores rather than
// disabling it, which is what §6.4 asks for.
//
// A `ref` may be `provider/model` or an existing combo name. The panel cannot tell which one it is looking
// at without resolving both sources, so it accepts any well-formed reference and lets the API report an
// unresolvable one. Claiming to validate that here would be a second, weaker resolver.

import { z } from 'zod';
import { optionalTimestamp } from './primitives';

// The three strategies §7.7 defines. Order matters: it is the order the select renders, from the simplest
// behaviour to the one that needs a second model.
export const COMBO_STRATEGIES = ['fallback', 'round_robin', 'fusion'] as const;

export type ComboStrategy = (typeof COMBO_STRATEGIES)[number];

// The §6.4 explaining copy, taken from the §7.7 semantics table so the panel describes the behaviour the
// router actually implements rather than a paraphrase of it.
export const COMBO_STRATEGY_EXPLANATIONS: Record<string, string> = {
	fallback: 'Try models in order until one succeeds.',
	round_robin: 'Distribute across models, keeping sticky_limit requests on one model first.',
	fusion: 'Send to several models and let judge_model write the final answer.'
};

export const COMBO_STRATEGY_LABELS: Record<string, string> = {
	fallback: 'Fallback',
	round_robin: 'Round robin',
	fusion: 'Fusion'
};

// Bounds from the API's own validation tags. `priority` is a combo entry's ordering key, which the API
// bounds at 0 to 100000, a wider range than an endpoint's 1 to 10000.
const COMBO_PRIORITY_MAX = 100000;
const COMBO_NAME_MAX = 120;
const COMBO_REF_MAX = 200;

// A combo name is a model string a client types, so it may not contain a slash: `a/b` is a provider/model
// reference and the two would be indistinguishable. The character set is the one the API accepts.
const COMBO_NAME_PATTERN = /^[A-Za-z0-9._-]+$/;

export const comboName = z
	.string()
	.transform((value) => value.trim())
	.refine((value) => value.length > 0, { message: 'A combo name is required.' })
	.refine((value) => value.length <= COMBO_NAME_MAX, {
		message: `Use ${COMBO_NAME_MAX} characters or fewer.`
	})
	.refine((value) => COMBO_NAME_PATTERN.test(value), {
		message: 'A combo name may only contain letters, numbers, dot, dash, and underscore.'
	});

// One model reference and its place in the order.
export const schemaComboModelEntry = z.object({
	ref: z
		.string()
		.transform((value) => value.trim())
		.refine((value) => value.length > 0, { message: 'A model reference is required.' })
		.refine((value) => value.length <= COMBO_REF_MAX, {
			message: `Use ${COMBO_REF_MAX} characters or fewer.`
		}),
	priority: z.coerce
		.number()
		.int({ message: 'Use a whole number for priority.' })
		.min(0, { message: 'Priority starts at 0.' })
		.max(COMBO_PRIORITY_MAX, { message: `Priority ends at ${COMBO_PRIORITY_MAX}.` })
});

export type ComboModelEntry = z.infer<typeof schemaComboModelEntry>;

// The read shape. `judge_model` is marked `omitempty` on the wire, so an absent field and an empty string
// both mean "no judge", and both normalize to the empty string here.
export const schemaCombo = z.object({
	id: z.string().min(1),
	name: z.string().min(1),
	strategy: z.string().min(1),
	sticky_limit: z.number().int(),
	judge_model: z
		.string()
		.nullish()
		.transform((value) => value ?? ''),
	models: z
		.array(schemaComboModelEntry)
		.nullish()
		.transform((value) => value ?? []),
	created_at: optionalTimestamp,
	updated_at: optionalTimestamp
});

export type Combo = z.infer<typeof schemaCombo>;

export const schemaComboList = z.object({
	data: z.array(schemaCombo),
	meta: z.object({
		page: z.number().int(),
		per_page: z.number().int(),
		total: z.number().int()
	})
});

export type ComboList = z.infer<typeof schemaComboList>;

// Whether a strategy reads `sticky_limit`. A pure predicate rather than a chain of literals, so a strategy
// the API adds later falls through the same rule instead of needing a new branch here.
export function usesStickyLimit(strategy: string): boolean {
	return strategy === 'round_robin';
}

// Whether a strategy reads `judge_model`.
export function usesJudgeModel(strategy: string): boolean {
	return strategy === 'fusion';
}

// The references listed more than once, in the order they first repeat. Every duplicate is reported, not
// just the first, because fixing one at a time is a loop the operator should not have to run.
//
// References are compared after trimming and case-sensitively: a ref is an identifier, so `OpenAI/GPT-4o`
// and `openai/gpt-4o` are two different strings the API would treat as two entries.
export function duplicateComboRefs(entries: { ref: string }[]): string[] {
	const seen = new Set<string>();
	const duplicates: string[] = [];

	for (const entry of entries) {
		const ref = entry.ref.trim();
		if (ref === '') continue;
		if (seen.has(ref)) {
			if (!duplicates.includes(ref)) duplicates.push(ref);
			continue;
		}
		seen.add(ref);
	}

	return duplicates;
}

// Moves one entry to a new position and renumbers the priorities to match.
//
// The order the operator sees is the order the API stores, expressed as `priority`, so a move has to
// renumber rather than swap: swapping two priorities leaves the list ordered but makes the numbers jump,
// and the next move would then be computed against numbers that no longer describe the positions. An index
// outside the list, or a move to where the entry already is, returns the list renumbered but unchanged,
// which is what a drag that lands on itself should do.
export function reorderComboModels(
	entries: ComboModelEntry[],
	from: number,
	to: number
): ComboModelEntry[] {
	const ordered = [...entries];

	if (from >= 0 && from < ordered.length && to >= 0 && to < ordered.length && from !== to) {
		const [moved] = ordered.splice(from, 1);
		ordered.splice(to, 0, moved);
	}

	return ordered.map((entry, index) => ({ ...entry, priority: index }));
}

// The strategy label used in a table cell. An unknown strategy renders verbatim, so one the API adds later
// shows its own name rather than a blank.
export function comboStrategyLabel(strategy: string): string {
	return COMBO_STRATEGY_LABELS[strategy] ?? strategy;
}

// The one-line description of what a combo resolves to, for the table's model column.
export function comboModelSummary(combo: Combo): string {
	const count = combo.models.length;
	return count === 1 ? '1 model' : `${count} models`;
}
