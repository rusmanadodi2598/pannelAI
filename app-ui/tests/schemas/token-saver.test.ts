// Token saver contract tests (docs/SPEC-UI/001-SPEC-UI.md §6.7, §7.6;
// docs/SPEC-API/001-SPEC-API.md §7.9; docs/SPEC-API/002-TOKEN-SAVER.md §4).
//
// Two rules here are the ones a screen cannot show by looking at it: a filter name the panel does not
// offer still parses, because dropping it would change a stored configuration, and a level outside the
// closed set fails, because a control that rendered it would be describing a bias the engine does not
// have. Both are tables, per docs/RULLES/TDD.md §2.5.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	buildTokenSaverBody,
	RTK_FILTERS,
	RTK_FILTER_NAMES,
	schemaHeadroomSection,
	schemaPonytailSection,
	schemaTokenSaver,
	schemaTokenSaverBody,
	TOKEN_SAVER_LEVELS,
	type TokenSaver
} from '$lib/schemas/token-saver';

function document_(overrides: Partial<TokenSaver> = {}): TokenSaver {
	return {
		rtk: { enabled: false, filters: [] },
		headroom: { enabled: false, url: '', compress_user_messages: false },
		ponytail: { enabled: false, level: 'full' },
		...overrides
	};
}

describe('schemaTokenSaver', () => {
	const valid = document_();

	const cases = [
		{ name: 'accepts the documented default document', input: valid, ok: true },
		{
			name: 'accepts an enabled group with an allowlist',
			input: document_({ rtk: { enabled: true, filters: ['git-diff', 'grep'] } }),
			ok: true
		},
		{
			name: 'accepts a filter name the panel does not offer',
			input: document_({ rtk: { enabled: true, filters: ['git-diff', 'future-filter'] } }),
			ok: true
		},
		{
			name: 'accepts every documented level',
			input: document_({ ponytail: { enabled: true, level: 'ultra' } }),
			ok: true
		},
		{
			name: 'rejects a level outside the closed set',
			input: { ...valid, ponytail: { enabled: true, level: 'extreme' } },
			ok: false
		},
		{ name: 'rejects a missing group', input: { rtk: valid.rtk }, ok: false },
		{
			name: 'rejects a missing level',
			input: { ...valid, ponytail: { enabled: true } },
			ok: false
		},
		{
			name: 'rejects a non-boolean enabled flag',
			input: { ...valid, rtk: { enabled: 'yes', filters: [] } },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaTokenSaver.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});

describe('the filter vocabulary', () => {
	it('names twelve filters', () => {
		expect(RTK_FILTER_NAMES).toHaveLength(12);
	});

	it('keeps the names unique', () => {
		expect(new Set(RTK_FILTER_NAMES).size).toBe(RTK_FILTER_NAMES.length);
	});

	it('states what each filter compacts, so the choice is informed', () => {
		for (const filter of RTK_FILTERS) {
			expect(filter.description.length, filter.name).toBeGreaterThan(0);
		}
	});
});

describe('schemaTokenSaverBody', () => {
	const valid = buildTokenSaverBody(document_());

	const cases = [
		{ name: 'accepts the documented default body', input: valid, ok: true },
		{
			name: 'accepts an http Headroom URL',
			input: {
				...valid,
				headroom: { enabled: true, url: 'http://localhost:8787', compress_user_messages: false }
			},
			ok: true
		},
		{
			name: 'accepts an empty Headroom URL',
			input: { ...valid, headroom: { enabled: true, url: '', compress_user_messages: false } },
			ok: true
		},
		{
			name: 'rejects a Headroom URL with no scheme',
			input: {
				...valid,
				headroom: { enabled: true, url: 'localhost:8787', compress_user_messages: false }
			},
			ok: false
		},
		{
			name: 'rejects a group the API does not accept',
			input: { ...valid, caveman: { enabled: false, level: 'full' } },
			ok: false
		},
		{ name: 'rejects a missing group', input: { rtk: valid.rtk }, ok: false },
		{
			name: 'rejects a level outside the closed set',
			input: { ...valid, ponytail: { enabled: true, level: 'extreme' } },
			ok: false
		},
		{
			name: 'rejects an empty filter name',
			input: { ...valid, rtk: { enabled: true, filters: [''] } },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaTokenSaverBody.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});

describe('schemaPonytailSection', () => {
	const cases = [
		{ name: 'accepts each documented level', input: { enabled: true, level: 'lite' }, ok: true },
		{ name: 'accepts the middle level', input: { enabled: false, level: 'full' }, ok: true },
		{ name: 'accepts the strongest level', input: { enabled: true, level: 'ultra' }, ok: true },
		{ name: 'rejects an unknown level', input: { enabled: true, level: 'turbo' }, ok: false },
		{ name: 'rejects a missing level', input: { enabled: true }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaPonytailSection.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});

	it('covers every level the constant declares', () => {
		for (const level of TOKEN_SAVER_LEVELS) {
			expect(schemaPonytailSection.safeParse({ enabled: true, level }).success, level).toBe(true);
		}
	});
});

describe('schemaHeadroomSection', () => {
	const cases = [
		{
			name: 'accepts an empty URL, which means the group is unset',
			input: { enabled: false, url: '', compress_user_messages: false },
			ok: true
		},
		{
			name: 'accepts an https URL',
			input: { enabled: true, url: 'https://headroom.example.com', compress_user_messages: true },
			ok: true
		},
		{
			name: 'rejects a URL with no scheme',
			input: { enabled: true, url: 'headroom.example.com', compress_user_messages: false },
			ok: false
		},
		{
			name: 'rejects a non-http scheme',
			input: { enabled: true, url: 'socks5://localhost:1080', compress_user_messages: false },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaHeadroomSection.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});

	it('strips the trailing slash the API also trims', () => {
		const parsed = schemaHeadroomSection.safeParse({
			enabled: true,
			url: 'http://localhost:8787/',
			compress_user_messages: false
		});

		expect(parsed.success && parsed.data.url).toBe('http://localhost:8787');
	});
});

describe('buildTokenSaverBody', () => {
	it('writes every group, because the route replaces the document', () => {
		const body = buildTokenSaverBody(
			document_({
				rtk: { enabled: true, filters: ['grep'] },
				ponytail: { enabled: false, level: 'lite' }
			})
		);

		expect(body).toEqual({
			rtk: { enabled: true, filters: ['grep'] },
			headroom: { enabled: false, url: '', compress_user_messages: false },
			ponytail: { enabled: false, level: 'lite' }
		});
	});

	it('copies the allowlist, so a later draft edit cannot change a body already built', () => {
		const draft = document_({ rtk: { enabled: true, filters: ['grep'] } });
		const body = buildTokenSaverBody(draft);
		draft.rtk.filters.push('ls');

		expect(body.rtk.filters).toEqual(['grep']);
	});
});
