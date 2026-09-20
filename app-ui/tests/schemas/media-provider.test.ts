// Media provider response-contract tests (docs/SPEC-UI/001-SPEC-UI.md §6.8, SPEC-API §7.10).
//
// The rules a reader cannot check by looking at the screen: which response fields are required, which
// are deliberately loose, and which of the two kind vocabularies travels where. The form side is
// `media-provider-form.test.ts`, so this file stays about what the API sends and what the panel can
// prove about a save before it makes one.

import { describe, expect, it } from 'vitest';
import {
	MEDIA_API_KINDS,
	MEDIA_KINDS,
	apiKindFor,
	isMediaKind,
	mediaBaseUrlRefusal,
	mediaKindLabel,
	panelKindFor,
	schemaMediaProvider,
	schemaMediaProviderDetail,
	schemaMediaProviderList,
	type MediaKindBlock
} from '$lib/schemas/media-provider';
import { forEachCase } from '../support/tables';

function mediaRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		provider_id: 'openai',
		provider_name: 'OpenAI',
		kind: 'tts',
		base_url: 'https://api.openai.com/v1/audio/speech',
		base_url_source: 'registry',
		default_model: 'tts-1',
		default_model_source: 'registry',
		endpoint_count: 2,
		models: [{ id: 'tts-1', name: 'TTS 1' }, { id: 'tts-1-hd' }],
		...overrides
	};
}

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

describe('the two kind vocabularies', () => {
	forEachCase(
		MEDIA_KINDS.map((kind) => ({ name: `${kind} round-trips through the API's spelling`, kind })),
		({ kind }) => {
			// The mapping has to be total in both directions: the panel's slug travels to the API as a body
			// field, and the API's kind travels back into a label. A one-way map would send `?kind=web`.
			expect(panelKindFor(apiKindFor(kind))).toBe(kind);
		}
	);

	it('covers the API set exactly, with no kind left unmapped', () => {
		// Both directions are total, so neither vocabulary can grow a member the other cannot name.
		expect(MEDIA_KINDS.length).toBe(MEDIA_API_KINDS.length);
		expect(new Set(MEDIA_KINDS.map(apiKindFor)).size).toBe(MEDIA_API_KINDS.length);
	});

	forEachCase(
		[
			{
				name: 'web is the panel name for the API kind search',
				kind: 'web' as const,
				api: 'search' as const
			},
			{
				name: 'embedding is spelled the same in both',
				kind: 'embedding' as const,
				api: 'embedding' as const
			},
			{ name: 'stt is spelled the same in both', kind: 'stt' as const, api: 'stt' as const }
		],
		({ kind, api }) => {
			expect(apiKindFor(kind)).toBe(api);
			expect(panelKindFor(api)).toBe(kind);
		}
	);

	forEachCase(
		[
			{ name: 'accepts every panel kind', value: 'video', ok: true },
			{
				name: 'refuses the API spelling of the web kind, which is not a panel slug',
				value: 'search',
				ok: false
			},
			{ name: 'refuses an uppercase spelling rather than folding it', value: 'TTS', ok: false },
			{ name: 'refuses a kind the panel does not have', value: 'chat', ok: false },
			{ name: 'refuses the empty string', value: '', ok: false }
		],
		({ value, ok }) => {
			expect(isMediaKind(value)).toBe(ok);
		}
	);

	it('gives every kind a label, and no label repeats', () => {
		const labels = MEDIA_KINDS.map(mediaKindLabel);
		expect(labels.every((label) => label.trim().length > 0)).toBe(true);
		expect(new Set(labels).size).toBe(labels.length);
	});
});

describe('schemaMediaProvider', () => {
	forEachCase(
		[
			{
				name: 'accepts a registry row with a declared model list',
				document: mediaRow(),
				ok: true
			},
			{
				name: 'accepts a row whose registry declares no base URL, which is a state and not an error',
				document: mediaRow({ base_url: '', base_url_source: 'registry' }),
				ok: true
			},
			{
				name: 'accepts a row overridden here, for both fields',
				document: mediaRow({
					base_url: 'http://127.0.0.1:8095/v1/audio/speech',
					base_url_source: 'override',
					default_model: 'tts-1-hd',
					default_model_source: 'override'
				}),
				ok: true
			},
			{
				name: 'accepts a kind that declares no models',
				document: mediaRow({ kind: 'image', models: [] }),
				ok: true
			},
			{
				name: 'accepts models that omit name and dimensions, which the API marks omitempty',
				document: mediaRow({
					models: [{ id: 'embed-small' }, { id: 'embed-large', dimensions: 3072 }]
				}),
				ok: true
			},
			{
				name: 'accepts a null model list, which is the other spelling of an empty one',
				document: mediaRow({ models: null }),
				ok: true
			},
			{
				name: 'accepts a zero endpoint count, which is the actionable case',
				document: mediaRow({ endpoint_count: 0 }),
				ok: true
			},
			{
				name: 'refuses a base URL source the panel does not know, because the self-hosted rule reads it',
				document: mediaRow({ base_url_source: 'inherited' }),
				ok: false
			},
			{
				name: 'refuses a default model source the panel does not know',
				document: mediaRow({ default_model_source: 'computed' }),
				ok: false
			},
			{
				name: 'refuses an unknown kind, because the API set is closed',
				document: mediaRow({ kind: 'chat' }),
				ok: false
			},
			{
				name: 'refuses a row with no provider id',
				document: mediaRow({ provider_id: '' }),
				ok: false
			},
			{
				name: 'refuses a row with no provider name, which would render a blank heading',
				document: mediaRow({ provider_name: '' }),
				ok: false
			},
			{
				name: 'refuses a negative endpoint count',
				document: mediaRow({ endpoint_count: -1 }),
				ok: false
			},
			{
				name: 'refuses a fractional endpoint count',
				document: mediaRow({ endpoint_count: 1.5 }),
				ok: false
			},
			{
				name: 'refuses a negative dimension count',
				document: mediaRow({ models: [{ id: 'embed-small', dimensions: -8 }] }),
				ok: false
			}
		],
		({ document, ok }) => {
			expect(schemaMediaProvider.safeParse(document).success).toBe(ok);
		}
	);

	it('keeps an unknown model field rather than dropping it, so a model the panel has no label for still renders', () => {
		const parsed = schemaMediaProvider.parse(mediaRow({ models: [{ id: 'x', dims: 4 }] }));
		expect(parsed.models).toEqual([{ id: 'x' }]);
	});
});

describe('schemaMediaProviderList and the detail answer', () => {
	forEachCase(
		[
			{ name: 'accepts a list with one row', document: { data: [mediaRow()] }, length: 1 },
			{
				name: 'accepts an empty list, which is the empty state',
				document: { data: [] },
				length: 0
			},
			{
				name: 'accepts a null list, which is the other spelling of empty',
				document: { data: null },
				length: 0
			}
		],
		({ document, length }) => {
			const parsed = schemaMediaProviderList.parse(document);
			expect(parsed.data.length).toBe(length);
		}
	);

	it('refuses a list whose row is malformed, so one bad row cannot render as a blank card', () => {
		expect(schemaMediaProviderList.safeParse({ data: [mediaRow({ kind: 'chat' })] }).success).toBe(
			false
		);
	});

	forEachCase(
		[
			{
				name: 'accepts a provider offering two kinds',
				document: {
					provider_id: 'openai',
					provider_name: 'OpenAI',
					media: [
						{ ...block({ kind: 'tts' }) },
						{ ...block({ kind: 'image', models: [{ id: 'gpt-image-1' }] }) }
					]
				},
				length: 2
			},
			{
				name: 'accepts a provider whose media list is null',
				document: { provider_id: 'ghost', provider_name: 'Ghost', media: null },
				length: 0
			}
		],
		({ document, length }) => {
			const parsed = schemaMediaProviderDetail.parse(document);
			expect(parsed.media.length).toBe(length);
		}
	);
});

describe('mediaBaseUrlRefusal', () => {
	// The server's sentence, from `service/media_provider.go`. The panel returns it verbatim so the
	// operator reads the same words whichever side caught the save.
	const sentence = (id: string, kind: string): string =>
		`provider ${id} has no ${kind} base_url; set one`;

	forEachCase(
		[
			{
				name: 'blocks the save it can prove would fail: the registry declares none and the field is empty',
				block: block({ kind: 'image', base_url: '', base_url_source: 'registry' }),
				baseUrl: '',
				refusal: sentence('together', 'image')
			},
			{
				name: 'blocks it when the field holds only whitespace, because the API trims before deciding',
				block: block({ kind: 'image', base_url: '', base_url_source: 'registry' }),
				baseUrl: '   ',
				refusal: sentence('together', 'image')
			},
			{
				name: 'allows a filled field, which is the fix the refusal asks for',
				block: block({ kind: 'image', base_url: '', base_url_source: 'registry' }),
				baseUrl: 'http://127.0.0.1:8095/v1/images/generations',
				refusal: null
			},
			{
				name: 'allows an empty field when the registry declares a value to fall back to',
				block: block({
					kind: 'tts',
					base_url: 'https://api.example.com/v1',
					base_url_source: 'registry'
				}),
				baseUrl: '',
				refusal: null
			},
			{
				name: 'allows an empty field over an override, because the registry value is not on the wire to check',
				block: block({
					kind: 'tts',
					base_url: 'http://127.0.0.1:8095/v1',
					base_url_source: 'override'
				}),
				baseUrl: '',
				refusal: null
			}
		],
		({ block: loaded, baseUrl, refusal }) => {
			expect(mediaBaseUrlRefusal({ providerId: 'together', block: loaded, baseUrl })).toBe(refusal);
		}
	);

	it('names the API kind rather than the panel slug, because the sentence quotes the server', () => {
		// The panel calls this kind `web` and the API calls it `search`. The refusal is a quotation, so it
		// has to use the word the server would use, or an operator matching it against a log would not find it.
		expect(
			mediaBaseUrlRefusal({
				providerId: 'brave-search',
				block: block({ kind: 'search', base_url: '', base_url_source: 'registry' }),
				baseUrl: ''
			})
		).toBe('provider brave-search has no search base_url; set one');
	});
});
