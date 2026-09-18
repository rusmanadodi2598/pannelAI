// Vision adapter schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.8 and the form in
// docs/SPEC-UI/001-SPEC-UI.md §6.4 tab 2.
//
// v1 ships the vision adapter only. The reference's pdf, audio-input, and video-input adapters are not
// ported, and §6.4 requires the screen to say so rather than render controls for them, which is why this
// module has no notion of a capability other than vision.
//
// The read and write shapes are deliberately the same, because §7.8 defines them that way: a PUT replaces
// the configuration, so the panel sends back exactly what it read plus the operator's edits. `models` is
// required rather than optional for the same reason: omitting it would be indistinguishable from clearing
// it, and "enabled with no models" is a state the screen warns about rather than a way to turn the adapter
// off.

import { z } from 'zod';
import { optionalTimestamp, stringList } from './primitives';

// A model reference in the adapter list. The API validates each entry as `provider/model` at 3 to 200
// characters, so the panel checks the shape it can check and lets the API resolve the reference.
export const visionModelRef = z
	.string()
	.transform((value) => value.trim())
	.refine((value) => value.length >= 3, { message: 'A model reference is at least 3 characters.' })
	.refine((value) => value.length <= 200, { message: 'Use 200 characters or fewer.' })
	.refine((value) => value.includes('/'), {
		message: 'Use the provider/model form, for example openai/gpt-4o.'
	});

export const VISION_MODELS_MAX = 64;

export const schemaVisionAdapter = z.object({
	enabled: z.boolean(),
	round_robin: z.boolean(),
	models: stringList,
	updated_at: optionalTimestamp
});

export type VisionAdapter = z.infer<typeof schemaVisionAdapter>;

// The form's own shape. `models` is a plain list of strings, which is what the API takes and returns.
export const schemaVisionAdapterForm = z.object({
	enabled: z.boolean(),
	roundRobin: z.boolean(),
	models: z.array(visionModelRef).max(VISION_MODELS_MAX, {
		message: `Use ${VISION_MODELS_MAX} models or fewer.`
	})
});

export type VisionAdapterForm = z.infer<typeof schemaVisionAdapterForm>;

// The request body §7.8 accepts. The keys are snake case because this is the wire shape, not the form.
export type VisionAdapterBody = {
	enabled: boolean;
	round_robin: boolean;
	models: string[];
};

// The form as the API's body. `models` is always present, even when empty, because the API marks it
// required and an omitted list would be read as an error rather than as "no models".
export function buildVisionAdapterBody(form: VisionAdapterForm): VisionAdapterBody {
	return {
		enabled: form.enabled,
		round_robin: form.roundRobin,
		models: [...form.models]
	};
}

// The stored configuration as the editor's initial form state.
export function visionAdapterToForm(adapter: VisionAdapter): VisionAdapterForm {
	return {
		enabled: adapter.enabled,
		roundRobin: adapter.round_robin,
		models: [...adapter.models]
	};
}

// The warning §6.4 requires: enabled with nothing selected means image requests are not adapted at all,
// which is a state that looks configured and is not.
//
// Both helpers below read the editor's own field names rather than the wire's, because the screen calls
// them on every keystroke and the form is the shape that is live while the operator edits.
export function visionAdapterWarning(adapter: {
	enabled: boolean;
	models: readonly string[];
}): string | null {
	if (!adapter.enabled) return null;
	if (adapter.models.length > 0) return null;
	return 'The adapter is enabled but no model is selected, so image requests are not adapted until you choose at least one.';
}

// The one-line state the screen shows above the form.
export function visionAdapterStatusText(adapter: {
	enabled: boolean;
	roundRobin: boolean;
	models: readonly string[];
}): string {
	if (!adapter.enabled) return 'Off';
	if (adapter.models.length === 0) return 'On, with no model selected';
	const rotation = adapter.roundRobin ? 'round robin' : 'in order';
	const count = adapter.models.length;
	return `On, ${count} ${count === 1 ? 'model' : 'models'} ${rotation}`;
}
