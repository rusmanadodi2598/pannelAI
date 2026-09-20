// Media override form tests (docs/SPEC-UI/001-SPEC-UI.md §6.8, SPEC-API §7.10).
//
// The form side of the contract: what the draft carries, what the PATCH body sends, and what the model
// selector offers. The response side is `media-provider.test.ts`, and the two share the fixture shape
// rather than the assertions.

import { describe, expect, it } from 'vitest';
import {
	MEDIA_DEFAULT_MODEL_VALUE,
	hasDeclaredModels,
	mediaBaseUrlHelp,
	mediaDraftFrom,
	mediaModelHelp,
	mediaModelOptions,
	mediaModelSelection,
	mediaOverrideBody,
	schemaMediaOverride,
	schemaMediaOverrideForm,
	undeclaredModel
} from '$lib/schemas/media-provider-form';
import type { MediaKindBlock } from '$lib/schemas/media-provider';
import { forEachCase } from '../support/tables';

function block(overrides: Partial<MediaKindBlock> = {}): MediaKindBlock {
	return {
		kind: 'tts',
		base_url: '',
		base_url_source: 'registry',
		default_model: '',
		default_model_source: 'registry',
		endpoint_count: 0,
		models: [],
		...overrides
	};
}

describe('mediaDraftFrom', () => {
	forEachCase(
		[
			{
				name: 'leaves both fields empty when both values come from the registry',
				block: block({ base_url: 'https://api.example.com/v1', default_model: 'tts-1' }),
				baseUrl: '',
				defaultModel: ''
			},
			{
				name: 'carries both values when both are overridden here',
				block: block({
					base_url: 'http://127.0.0.1:8095/v1',
					base_url_source: 'override',
					default_model: 'tts-1-hd',
					default_model_source: 'override'
				}),
				baseUrl: 'http://127.0.0.1:8095/v1',
				defaultModel: 'tts-1-hd'
			},
			{
				name: 'carries only the overridden base URL, because each field reports its own source',
				block: block({
					base_url: 'http://127.0.0.1:8095/v1',
					base_url_source: 'override',
					default_model: 'tts-1',
					default_model_source: 'registry'
				}),
				baseUrl: 'http://127.0.0.1:8095/v1',
				defaultModel: ''
			},
			{
				name: 'carries only the overridden model, which is the other half of the same rule',
				block: block({
					base_url: 'https://api.example.com/v1',
					base_url_source: 'registry',
					default_model: 'tts-1-hd',
					default_model_source: 'override'
				}),
				baseUrl: '',
				defaultModel: 'tts-1-hd'
			}
		],
		({ block: loaded, baseUrl, defaultModel }) => {
			// The draft is the override, never the resolved value: pre-filling the registry's value would
			// write an override on an untouched save and change the source without changing behaviour.
			expect(mediaDraftFrom(loaded)).toEqual({ baseUrl, defaultModel });
		}
	);
});

describe('mediaOverrideBody', () => {
	forEachCase(
		[
			{
				name: 'sends search for the kind the panel calls web',
				kind: 'web' as const,
				api: 'search'
			},
			{ name: 'sends tts unchanged', kind: 'tts' as const, api: 'tts' },
			{ name: 'sends embedding unchanged', kind: 'embedding' as const, api: 'embedding' }
		],
		({ kind, api }) => {
			expect(mediaOverrideBody(kind, { baseUrl: '', defaultModel: '' })).toEqual({
				kind: api,
				base_url: '',
				default_model: ''
			});
		}
	);

	it('passes an empty field through as an empty string, which is how an override is undone', () => {
		// The API reads an empty value as "use the registry's", so the panel must send it rather than omit it:
		// an omitted field would leave the stored override in place.
		expect(mediaOverrideBody('tts', { baseUrl: '', defaultModel: 'tts-1-hd' })).toEqual({
			kind: 'tts',
			base_url: '',
			default_model: 'tts-1-hd'
		});
	});
});

describe('schemaMediaOverrideForm', () => {
	forEachCase(
		[
			{
				name: 'accepts an empty base URL, which means the registry value applies',
				baseUrl: '',
				ok: true
			},
			{
				name: 'accepts an absolute http URL',
				baseUrl: 'http://127.0.0.1:8095/v1/audio/speech',
				ok: true
			},
			{
				name: 'accepts an absolute https URL',
				baseUrl: 'https://api.example.com/v1/audio/speech',
				ok: true
			},
			{ name: 'refuses a URL with no scheme', baseUrl: 'api.example.com/v1', ok: false },
			{
				name: 'refuses a scheme the API cannot dial',
				baseUrl: 'ftp://api.example.com/v1',
				ok: false
			},
			{
				name: 'refuses whitespace alone, which the API would trim to nothing',
				baseUrl: '   ',
				ok: false
			},
			{ name: 'refuses a relative path', baseUrl: '/v1/audio/speech', ok: false }
		],
		({ baseUrl, ok }) => {
			expect(schemaMediaOverrideForm.safeParse({ baseUrl, defaultModel: '' }).success).toBe(ok);
		}
	);

	it('strips a trailing slash, so two spellings of one address are one value', () => {
		const parsed = schemaMediaOverrideForm.parse({
			baseUrl: 'https://api.example.com/v1/audio/speech/',
			defaultModel: ''
		});
		expect(parsed.baseUrl).toBe('https://api.example.com/v1/audio/speech');
	});

	forEachCase(
		[
			{
				name: 'accepts an empty model, which means the registry default applies',
				length: 0,
				ok: true
			},
			{ name: 'accepts a model id at the API bound of 200', length: 200, ok: true },
			{ name: 'refuses a model id one past the API bound', length: 201, ok: false }
		],
		({ length, ok }) => {
			expect(
				schemaMediaOverrideForm.safeParse({ baseUrl: '', defaultModel: 'x'.repeat(length) }).success
			).toBe(ok);
		}
	);
});

describe('schemaMediaOverride', () => {
	it('accepts the body the API documents, with an empty value for each undoable field', () => {
		expect(schemaMediaOverride.parse({ kind: 'search', base_url: '', default_model: '' })).toEqual({
			kind: 'search',
			base_url: '',
			default_model: ''
		});
	});

	forEachCase(
		[
			{
				name: 'refuses the panel slug for the web kind, because the body carries the API spelling',
				body: { kind: 'web', base_url: '', default_model: '' },
				ok: false
			},
			{
				name: 'refuses a kind the API does not have',
				body: { kind: 'chat', base_url: '', default_model: '' },
				ok: false
			},
			{
				name: 'refuses a field the API does not accept, rather than letting it be dropped',
				body: { kind: 'tts', base_url: '', default_model: '', provider_id: 'openai' },
				ok: false
			},
			{
				name: 'accepts the three fields and nothing else',
				body: { kind: 'tts', base_url: 'http://127.0.0.1:8095/v1', default_model: 'tts-1' },
				ok: true
			}
		],
		({ body, ok }) => {
			expect(schemaMediaOverride.safeParse(body).success).toBe(ok);
		}
	);
});

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
				help: 'Set here. Clear it to go back to the value the registry declares.'
			},
			{
				name: 'says the registry declares none, which is the case the save is blocked in',
				block: block({ base_url: '', base_url_source: 'registry' }),
				help: 'The registry declares no base URL for this kind, so this provider needs one here.'
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
				help: 'This service declares no models, so there is nothing to choose. The registry default applies.'
			},
			{
				name: 'describes the choice when models are declared',
				block: block({ models: [{ id: 'tts-1' }] }),
				help: 'The registry default, or one of the models this service declares.'
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
