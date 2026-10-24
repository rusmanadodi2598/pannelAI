// What the override form's two fields offer and say (docs/SPEC-UI/001-SPEC-UI.md §6.8).
//
// The draft, the body and the schemas are `media-provider-form.test.ts`; these are the two things the card
// shows beside its fields: what the model selector holds, and the sentence each field points at. Both are
// read from the kind block rather than from a render, because the selector's order and the hint's value are
// the API's data, not markup.

import { describe, expect, it } from 'vitest';
import {
	MEDIA_DEFAULT_MODEL_VALUE,
	hasDeclaredModels,
	mediaBaseUrlHelp,
	mediaModelHelp,
	mediaModelOptions,
	mediaModelSelection,
	undeclaredModel
} from '$lib/schemas/media-provider-form';
import { forEachCase } from '../support/tables';
import { mediaBlock as block } from '../support/media-stub';

describe('the default model selector', () => {
	const declared = block({
		models: [{ id: 'tts-1', name: 'TTS 1' }, { id: 'tts-1-hd' }, { id: 'tts-2', name: 'tts-2' }]
	});

	it('offers the registry default first, then every declared model in the order the API sent them', () => {
		expect(mediaModelOptions(declared)).toEqual([
			{ value: MEDIA_DEFAULT_MODEL_VALUE, label: 'Use the registry default' },
			{ value: 'tts-1', label: 'TTS 1 (tts-1)' },
			{ value: 'tts-1-hd', label: 'tts-1-hd' },
			// The API sends `name` equal to the id here, and repeating it would read as a rendering fault.
			{ value: 'tts-2', label: 'tts-2' }
		]);
	});

	it('offers only the default when the kind declares no models, because there is nothing to choose', () => {
		expect(mediaModelOptions(block())).toEqual([
			{ value: MEDIA_DEFAULT_MODEL_VALUE, label: 'Use the registry default' }
		]);
	});

	forEachCase(
		[
			{ name: 'reports no declared models for an empty list', models: [], expected: false },
			{
				name: 'reports declared models for a non-empty list',
				models: [{ id: 'tts-1' }],
				expected: true
			}
		],
		({ models, expected }) => {
			expect(hasDeclaredModels(block({ models }))).toBe(expected);
		}
	);

	forEachCase(
		[
			{
				name: 'shows the registry default when the model comes from the registry',
				block: block({
					default_model: 'tts-1',
					default_model_source: 'registry',
					models: [{ id: 'tts-1' }]
				}),
				selection: MEDIA_DEFAULT_MODEL_VALUE
			},
			{
				name: 'shows the stored override when the service still declares it',
				block: block({
					default_model: 'tts-1-hd',
					default_model_source: 'override',
					models: [{ id: 'tts-1' }, { id: 'tts-1-hd' }]
				}),
				selection: 'tts-1-hd'
			},
			{
				name: 'shows the registry default when the service no longer declares the stored override',
				block: block({
					default_model: 'tts-0',
					default_model_source: 'override',
					models: [{ id: 'tts-1' }]
				}),
				selection: MEDIA_DEFAULT_MODEL_VALUE
			},
			{
				name: 'shows the registry default when the kind declares no models at all',
				block: block({ default_model: 'tts-0', default_model_source: 'override', models: [] }),
				selection: MEDIA_DEFAULT_MODEL_VALUE
			}
		],
		({ block: loaded, selection }) => {
			expect(mediaModelSelection(loaded)).toBe(selection);
		}
	);

	forEachCase(
		[
			{
				name: 'names nothing when the model comes from the registry',
				block: block({
					default_model: 'tts-1',
					default_model_source: 'registry',
					models: [{ id: 'tts-1' }]
				}),
				undeclared: null
			},
			{
				name: 'names nothing when the stored override is still declared',
				block: block({
					default_model: 'tts-1',
					default_model_source: 'override',
					models: [{ id: 'tts-1' }]
				}),
				undeclared: null
			},
			{
				name: 'names the stored override the service dropped, so the operator can see why the selector reset',
				block: block({
					default_model: 'tts-0',
					default_model_source: 'override',
					models: [{ id: 'tts-1' }]
				}),
				undeclared: 'tts-0'
			},
			{
				name: 'names nothing when there is no stored model at all',
				block: block({
					default_model: '',
					default_model_source: 'override',
					models: [{ id: 'tts-1' }]
				}),
				undeclared: null
			}
		],
		({ block: loaded, undeclared }) => {
			expect(undeclaredModel(loaded)).toBe(undeclared);
		}
	);
});

describe('the field hints', () => {
	forEachCase(
		[
			{
				name: 'tells the operator how to undo an override',
				block: block({ base_url: 'http://127.0.0.1:8095/v1', base_url_source: 'override' }),
				help: 'Set here. Clear it to use the registry value.'
			},
			{
				name: 'says the registry declares none, which is the case the save is blocked in',
				block: block({ base_url: '', base_url_source: 'registry' }),
				help: 'This kind has no base URL. Set one here.'
			},
			{
				name: 'names the registry value it would fall back to',
				block: block({ base_url: 'https://api.example.com/v1', base_url_source: 'registry' }),
				help: 'The registry declares https://api.example.com/v1. Leave this empty to use it.'
			}
		],
		({ block: loaded, help }) => {
			expect(mediaBaseUrlHelp(loaded)).toBe(help);
		}
	);

	forEachCase(
		[
			{
				name: 'says there is nothing to choose when the service declares no models',
				block: block(),
				help: 'No models here, so the registry default applies.'
			},
			{
				name: 'describes the choice when models are declared',
				block: block({ models: [{ id: 'tts-1' }] }),
				help: 'The registry default, or a model this service declares.'
			}
		],
		({ block: loaded, help }) => {
			expect(mediaModelHelp(loaded)).toBe(help);
		}
	);

	it('never promises a fallback when a provider fails, because the API implements none', () => {
		// §6.8 forbids the claim. The hints talk about where a value comes from, which is a real fallback
		// the API does implement, and say nothing about failure.
		const hints = [
			mediaBaseUrlHelp(block({ base_url: 'https://a.example.com', base_url_source: 'registry' })),
			mediaBaseUrlHelp(block()),
			mediaBaseUrlHelp(block({ base_url_source: 'override', base_url: 'https://b.example.com' })),
			mediaModelHelp(block()),
			mediaModelHelp(block({ models: [{ id: 'tts-1' }] }))
		];

		for (const hint of hints) {
			expect(hint, `a hint promises a fallback: ${hint}`).not.toMatch(
				/fall ?back|retry|fail over/i
			);
		}
	});
});
