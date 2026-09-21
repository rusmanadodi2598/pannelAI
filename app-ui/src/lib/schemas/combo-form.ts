// The combo editor's form shape, its validation rules, and the mapping onto the API's request body.
//
// Split from `combo.ts` so the read contracts and the editor's own shape stay separate files: the editor
// carries cross-field rules that the response shape has no opinion about, and a reader looking for "what
// does the API return" should not have to walk past them.
//
// The strategy decides the shape here too. `fallback` normalizes `sticky_limit` to 1 and refuses a
// `judge_model`, `round_robin` requires `sticky_limit` of at least 1, and `fusion` requires a
// `judge_model`. The API enforces the same rules in its domain layer, so this is a copy of a rule rather
// than a second opinion about it, and it exists so a save is refused before the round trip.

import { z } from 'zod';
import {
	COMBO_STRATEGIES,
	comboName,
	duplicateComboRefs,
	schemaComboModelEntry,
	usesJudgeModel,
	usesStickyLimit,
	type Combo,
	type ComboStrategy
} from './combo';

// Bounds from the API's own validation tags (SPEC-API §7.7).
const COMBO_STICKY_MAX = 100000;
const COMBO_REF_MAX = 200;
const COMBO_MODELS_MAX = 64;

// The editor's own shape. Cross-field rules live in one `superRefine` because they are properties of the
// combo as a whole, not of any single field: whether `judgeModel` is required depends on `strategy`.
export const schemaComboForm = z
	.object({
		name: comboName,
		strategy: z.enum(COMBO_STRATEGIES),
		models: z
			.array(schemaComboModelEntry)
			.min(1, { message: 'A combo needs at least one model.' })
			.max(COMBO_MODELS_MAX, { message: `Use ${COMBO_MODELS_MAX} models or fewer.` }),
		stickyLimit: z.coerce
			.number()
			.int({ message: 'Use a whole number for the sticky limit.' })
			.min(0, { message: 'The sticky limit starts at 0.' })
			.max(COMBO_STICKY_MAX, { message: `The sticky limit ends at ${COMBO_STICKY_MAX}.` }),
		judgeModel: z
			.string()
			.transform((value) => value.trim())
			.refine((value) => value.length <= COMBO_REF_MAX, {
				message: `Use ${COMBO_REF_MAX} characters or fewer.`
			})
	})
	.superRefine((form, ctx) => {
		const duplicates = duplicateComboRefs(form.models);
		if (duplicates.length > 0) {
			ctx.addIssue({
				code: 'custom',
				path: ['models'],
				message: `A model reference is listed twice: ${duplicates.join(', ')}.`
			});
		}

		if (usesJudgeModel(form.strategy) && form.judgeModel === '') {
			ctx.addIssue({
				code: 'custom',
				path: ['judgeModel'],
				message: 'A fusion combo needs a judge model to write the final answer.'
			});
		}

		if (usesStickyLimit(form.strategy) && form.stickyLimit < 1) {
			ctx.addIssue({
				code: 'custom',
				path: ['stickyLimit'],
				message:
					'A round robin combo keeps at least one request on a model, so the sticky limit starts at 1.'
			});
		}
	});

export type ComboForm = z.infer<typeof schemaComboForm>;

// The request body §7.7 accepts. Create and patch carry the same shape, so a patch is a full replace
// rather than a delta, which is why every field is required here.
export type ComboBody = {
	name: string;
	strategy: string;
	models: { ref: string; priority: number }[];
	sticky_limit?: number;
	judge_model?: string;
};

// The form as the API's body. Each optional field is emitted only when its strategy reads it, so a combo
// that changed strategy cannot carry a value the new strategy refuses: a leftover `judge_model` on a
// `fallback` combo, or a leftover `sticky_limit` on a `fusion` one, is a save the API would reject.
export function buildComboBody(form: ComboForm): ComboBody {
	const body: ComboBody = {
		name: form.name,
		strategy: form.strategy,
		models: form.models.map((entry) => ({ ref: entry.ref, priority: entry.priority }))
	};

	if (usesStickyLimit(form.strategy)) body.sticky_limit = form.stickyLimit;
	if (usesJudgeModel(form.strategy)) body.judge_model = form.judgeModel;

	return body;
}

// A stored combo as the editor's initial form state. `sticky_limit` is seeded with 1 for a strategy that
// ignores it, so the field holds a value the rule accepts if the operator switches to `round_robin`,
// which is the only way the hidden field can become visible.
export function comboToForm(combo: Combo): ComboForm {
	return {
		name: combo.name,
		strategy: (COMBO_STRATEGIES as readonly string[]).includes(combo.strategy)
			? (combo.strategy as ComboStrategy)
			: 'fallback',
		models: combo.models.map((entry) => ({ ref: entry.ref, priority: entry.priority })),
		stickyLimit: usesStickyLimit(combo.strategy) ? combo.sticky_limit : 1,
		judgeModel: combo.judge_model
	};
}

// Whether the editor holds an unsaved change. `baseline` is the form as it was seeded, so a save or a
// re-seed clears this and one keystroke sets it. The models are compared field by field rather than by
// reference, because the rows are replaced rather than mutated by the child that edits them.
export function comboFormDirty(baseline: ComboForm, form: ComboForm): boolean {
	if (
		baseline.name !== form.name ||
		baseline.strategy !== form.strategy ||
		baseline.stickyLimit !== form.stickyLimit ||
		baseline.judgeModel !== form.judgeModel ||
		baseline.models.length !== form.models.length
	) {
		return true;
	}

	return baseline.models.some((entry, index) => {
		const current = form.models[index];
		return (
			current === undefined || entry.ref !== current.ref || entry.priority !== current.priority
		);
	});
}
