// What an answer accepts, and what the panel answers before it sends one (docs/SPEC-UI/001-SPEC-UI.md
// §6.8, SPEC-API §7.10).
//
// Both subjects are about a response rather than a screen: one row's contract is `media-provider.test.ts`,
// and these are the list and the detail answer that wrap it, plus the refusal the panel raises itself when
// the row already proves the save would fail. The refusal quotes the server's sentence, so the operator
// reads the same words whichever side caught it.

import { describe, expect, it } from 'vitest';
import {
	mediaBaseUrlRefusal,
	schemaMediaProviderDetail,
	schemaMediaProviderList
} from '$lib/schemas/media-provider';
import { forEachCase } from '../support/tables';
import { mediaBlock as block, mediaRow } from '../support/media-stub';

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
