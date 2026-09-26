// Custom model contracts for docs/SPEC-API/001-SPEC-API.md §7.6 and docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// A custom model is one the operator declares that the registry does not carry. The merged catalog
// renders it with `source: custom`, and the API validates the provider against the registry, so a custom
// row is always a model under a provider the gateway can route to.
//
// The form mirrors only the rules the panel can prove from the API's own contract: the DTO's bounds
// (`model_id` 1 to 200, `display_name` 1 to 120, at most 32 capabilities of at most 64 characters each)
// and the domain's refusal of whitespace inside a model id. It adds no rule of its own, because a panel
// rule the gateway does not share would refuse a write the API would have accepted.

import { z } from 'zod';
import { optionalLabel, rfc3339Timestamp, stringList } from './primitives';

// The DTO's bounds for one capability entry and for the list itself.
export const CAPABILITY_MAX_ENTRIES = 32;
export const CAPABILITY_MAX_LENGTH = 64;

// A model id segment: the part after the provider in `provider/model`. SPEC-UI §7.2 bounds a model ref as
// trimmed with no whitespace inside a segment, and app-serv's domain refuses whitespace and caps a segment
// at 200 characters (`modelSegmentMax`), so both rules are stated here. The ceiling is the API's, not a
// panel-invented bound, so the panel cannot refuse an id the gateway would have accepted.
//
// It lives with the resource rather than in `primitives.ts` for the reason `combo.ts` keeps its own ref
// rule: the model-ref class is the one field class whose rules differ by resource, and the shared file is
// at its own size limit. A second screen that names a model id should import this one.
export const MODEL_SEGMENT_MAX = 200;

export const modelSegment = z
	.string()
	.transform((value) => value.trim())
	.refine((value) => value.length > 0, { message: 'A model id is required.' })
	.refine((value) => value.length <= MODEL_SEGMENT_MAX, {
		message: `Use ${MODEL_SEGMENT_MAX} characters or fewer.`
	})
	.refine((value) => !/\s/.test(value), { message: 'A model id cannot contain spaces.' });

// One custom model row. The row carries the provider's model id and the display name separately because
// the catalog shows both, and `id` is the `mdl_` row id the delete route takes.
export const schemaCustomModel = z.object({
	id: z.string().min(1),
	provider_id: z.string().min(1),
	model_id: z.string().min(1),
	display_name: z.string(),
	capabilities: stringList,
	// The levels this model accepts when appended to its name (SPEC-API §7.14). Absent when
	// the registry knows none for the id, which is the answer that keeps a suffix off a
	// model whose upstream would refuse it.
	thinking_levels: stringList.optional(),
	created_at: rfc3339Timestamp
});

export type CustomModel = z.infer<typeof schemaCustomModel>;

// The list body. Not paginated: it is a table the operator curated by hand, so the bound is what they
// added rather than a customer's data set (§7.6).
export const schemaCustomModelList = z.object({
	data: z.array(schemaCustomModel)
});

export type CustomModelList = z.infer<typeof schemaCustomModelList>;

// Splits the typed field into the list the API takes: commas separate entries, surrounding whitespace
// goes, blank entries are dropped, and duplicates collapse with the first occurrence keeping its place.
// Dropping a blank is what keeps a trailing comma from becoming an empty string the DTO would refuse.
export function parseCapabilities(text: string): string[] {
	const seen = new Set<string>();
	const out: string[] = [];
	for (const part of text.split(',')) {
		const entry = part.trim();
		if (entry === '' || seen.has(entry)) continue;
		seen.add(entry);
		out.push(entry);
	}
	return out;
}

// The reverse, so a stored row and a typed one render the same way.
export function capabilitiesText(list: readonly string[]): string {
	return list.join(', ');
}

// What an operator types. `model_id` is the one field the API requires; the display name is optional here
// even though the wire insists on a non-empty one, because the reference adds a model with its id alone
// (draft 019 D2) and the body fills the name from the id. Capabilities are optional, and the field is free
// text because the API accepts any value and exposes no vocabulary.
export const schemaCustomModelForm = z.strictObject({
	model_id: modelSegment,
	display_name: optionalLabel,
	capabilities: z
		.string()
		.refine((text) => parseCapabilities(text).length <= CAPABILITY_MAX_ENTRIES, {
			message: `Use ${CAPABILITY_MAX_ENTRIES} capabilities or fewer.`
		})
		.refine(
			(text) => parseCapabilities(text).every((entry) => entry.length <= CAPABILITY_MAX_LENGTH),
			{ message: `Each capability must be ${CAPABILITY_MAX_LENGTH} characters or fewer.` }
		)
});

export type CustomModelForm = z.infer<typeof schemaCustomModelForm>;

export function customModelDraftEmpty(): CustomModelForm {
	return { model_id: '', display_name: '', capabilities: '' };
}

// The POST body. The provider is the page's, not the operator's: the screen is one provider's detail, so
// the field is fixed and stated rather than offered as a control that could name another provider.
export type CreateCustomModelBody = {
	provider_id: string;
	model_id: string;
	display_name: string;
	capabilities: string[];
};

// The display name is filled from the model id when the field was left blank: the wire refuses an empty one
// (`schema/model.go:47`), and the id is what the row would show anyway (`customModelLabel`), so asking for a
// second name would be a field with no consequence (draft 019 D2).
export function customModelBody(providerId: string, form: CustomModelForm): CreateCustomModelBody {
	const name = form.display_name.trim();
	return {
		provider_id: providerId,
		model_id: form.model_id,
		display_name: name === '' ? form.model_id : name,
		capabilities: parseCapabilities(form.capabilities)
	};
}

// One provider's rows, for display. The list route reads every provider's rows, and this narrows it the
// same way the catalog narrows by provider.
export function providerCustomModels(
	rows: readonly CustomModel[],
	providerId: string
): CustomModel[] {
	return rows.filter((row) => row.provider_id === providerId);
}

// The name a row is listed under. The API requires a display name, so this is the display name; the
// fallback covers a row stored before that rule, which would otherwise render a blank cell.
export function customModelLabel(row: CustomModel): string {
	return row.display_name.trim() === '' ? row.model_id : row.display_name;
}
