// The media override form: the draft an operator edits, the body §7.10 accepts, and the options the
// default model selector offers.
//
// The draft carries the override, not the resolved value. §7.10's body semantics make an empty
// `base_url` mean "use the registry's", which is how an override is undone, so a field pre-filled
// with the registry's value would write an override on an untouched save and change the source
// without changing behaviour. The registry's value is shown beside the field instead, as a fact.
//
// The selector is where the model rule is enforced. The API refuses a model the kind does not
// declare, but only when the kind declares at least one, so the panel offers exactly the declared
// set plus the empty default and an invalid value becomes structurally impossible. That is why no
// message exists for the model the way one does for `base_url`, which is free text.

import { z } from 'zod';
import { optionalAbsoluteUrl } from './primitives';
import {
	apiKindFor,
	schemaMediaApiKind,
	type MediaKind,
	type MediaKindBlock,
	type MediaModel
} from './media-provider';

export type MediaOverrideDraft = {
	/** The override. Empty means "use the registry's value". */
	baseUrl: string;
	/** The override. Empty means "use the registry's default". */
	defaultModel: string;
};

export function emptyMediaDraft(): MediaOverrideDraft {
	return { baseUrl: '', defaultModel: '' };
}

export function mediaDraftFrom(block: MediaKindBlock): MediaOverrideDraft {
	return {
		baseUrl: block.base_url_source === 'override' ? block.base_url : '',
		defaultModel: block.default_model_source === 'override' ? block.default_model : ''
	};
}

export const schemaMediaOverrideForm = z.strictObject({
	baseUrl: optionalAbsoluteUrl,
	defaultModel: z.string().max(200, { message: 'Use 200 characters or fewer.' })
});

export type MediaOverrideForm = z.infer<typeof schemaMediaOverrideForm>;

// The PATCH body §7.10 accepts, mirroring `schema.MediaOverrideRequest`. Strict, so a field the panel
// invents fails here rather than being dropped by the API. The kind is in the body because the route
// addresses a provider and the panel's page is per kind.
export const schemaMediaOverride = z.strictObject({
	kind: schemaMediaApiKind,
	base_url: z.string().max(2048, { message: 'Use 2048 characters or fewer.' }),
	default_model: z.string().max(200, { message: 'Use 200 characters or fewer.' })
});

export type MediaOverride = z.infer<typeof schemaMediaOverride>;

export function mediaOverrideBody(kind: MediaKind, form: MediaOverrideForm): MediaOverride {
	return {
		kind: apiKindFor(kind),
		base_url: form.baseUrl,
		default_model: form.defaultModel
	};
}

/** The empty value of the default model selector, which is the registry's own default. */
export const MEDIA_DEFAULT_MODEL_VALUE = '';

export type MediaModelOption = { value: string; label: string };

/** True when the kind declares models, which is the only case a selector can offer a real choice. */
export function hasDeclaredModels(block: MediaKindBlock): boolean {
	return block.models.length > 0;
}

function isDeclared(block: MediaKindBlock, model: string): boolean {
	return block.models.some((declared) => declared.id === model);
}

function modelLabel(model: MediaModel): string {
	// The API omits `name` when it equals the id or is unset, so a label that repeated the id twice
	// would read as a rendering fault rather than a name.
	return model.name !== undefined && model.name !== '' && model.name !== model.id
		? `${model.name} (${model.id})`
		: model.id;
}

/**
 * The options the selector offers: the registry's default, then every declared model.
 *
 * An undeclared stored override is deliberately not an option. It cannot be saved again (the API
 * refuses it), so offering it would be a dead end; `undeclaredModel` names it beside the selector
 * instead, which is what lets the operator clear it.
 */
export function mediaModelOptions(block: MediaKindBlock): MediaModelOption[] {
	return [
		{ value: MEDIA_DEFAULT_MODEL_VALUE, label: 'Use the registry default' },
		...block.models.map((model) => ({ value: model.id, label: modelLabel(model) }))
	];
}

/**
 * The value the selector shows.
 *
 * A stored override the service no longer declares resolves to the empty default, so the control
 * agrees with what the operator can actually do about it: clearing it. Showing the undeclared value
 * as selected would present a choice that cannot be saved.
 */
export function mediaModelSelection(block: MediaKindBlock): string {
	if (block.default_model_source !== 'override') return MEDIA_DEFAULT_MODEL_VALUE;
	if (block.default_model === '') return MEDIA_DEFAULT_MODEL_VALUE;

	return isDeclared(block, block.default_model) ? block.default_model : MEDIA_DEFAULT_MODEL_VALUE;
}

/**
 * The stored override that the service no longer declares, or null.
 *
 * This happens when the registry drops a model after an override named it, and it is worth naming:
 * without it the screen would say "use the registry default" while the API resolves something else,
 * and the operator would have no way to see why.
 */
export function undeclaredModel(block: MediaKindBlock): string | null {
	if (block.default_model_source !== 'override') return null;
	if (block.default_model === '') return null;

	return isDeclared(block, block.default_model) ? null : block.default_model;
}

/**
 * What the base URL field's hint says.
 *
 * The three cases are the three states the source can be in, and each names a different action: an
 * override can be cleared, a missing registry value has to be filled, and a present one is already
 * being used. None of them promises a fallback when the provider fails, because the API implements
 * none (§6.8).
 */
export function mediaBaseUrlHelp(block: MediaKindBlock): string {
	if (block.base_url_source === 'override') {
		return 'Set here. Clear it to go back to the value the registry declares.';
	}

	if (block.base_url === '') {
		return 'The registry declares no base URL for this kind, so this provider needs one here.';
	}

	return `The registry declares ${block.base_url}. Leave this empty to use it.`;
}

/** What the default model field says when it is a selector, and why when it is not. */
export function mediaModelHelp(block: MediaKindBlock): string {
	if (!hasDeclaredModels(block)) {
		return 'This service declares no models, so there is nothing to choose. The registry default applies.';
	}

	return 'The registry default, or one of the models this service declares.';
}
