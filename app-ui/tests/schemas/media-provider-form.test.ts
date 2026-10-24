// Media override form tests (docs/SPEC-UI/001-SPEC-UI.md §6.8, SPEC-API §7.10).
//
// The form side of the contract: what the draft carries and what the PATCH body sends. The response side is
// `media-provider.test.ts`, and what the card shows beside its two fields is in
// `media-provider-form-fields.test.ts`.

import { describe, expect, it } from 'vitest';
import {
	mediaDraftFrom,
	mediaOverrideBody,
	schemaMediaOverride,
	schemaMediaOverrideForm
} from '$lib/schemas/media-provider-form';
import { forEachCase } from '../support/tables';
import { mediaBlock as block } from '../support/media-stub';

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
