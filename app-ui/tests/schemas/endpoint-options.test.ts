// Provider option list tests (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
//
// The list is what the operator picks a provider from, so the cases are the ones that would make the select
// unusable rather than merely untidy: a provider repeated across rows, a provider the registry does not
// name, and a provider the address names that no read returned.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { endpointRow } from '../support/endpoint-stub';
import {
	OPTION_PAGE_SIZE,
	toProviderOptions,
	withSelectedProvider,
	type ProviderOption
} from '$lib/schemas/endpoint-options';
import type { Endpoint } from '$lib/schemas/endpoint';

function rows(): Endpoint[] {
	return [
		endpointRow({ id: 'ep_1', provider_id: 'openai', provider_name: 'OpenAI' }),
		endpointRow({ id: 'ep_2', provider_id: 'anthropic', provider_name: 'Anthropic' }),
		endpointRow({ id: 'ep_3', provider_id: 'openai', provider_name: 'OpenAI' })
	] as Endpoint[];
}

describe('toProviderOptions', () => {
	it('lists each provider once, sorted by the name the operator reads', () => {
		expect(toProviderOptions(rows())).toEqual([
			{ id: 'anthropic', name: 'Anthropic' },
			{ id: 'openai', name: 'OpenAI' }
		]);
	});

	it('names a provider by its id when the registry does not name it', () => {
		const unnamed = [
			endpointRow({ provider_id: 'local_box', provider_name: undefined })
		] as Endpoint[];

		expect(toProviderOptions(unnamed)).toEqual([{ id: 'local_box', name: 'local_box' }]);
	});

	it('offers nothing for no rows, rather than an option with no value', () => {
		expect(toProviderOptions([])).toEqual([]);
	});

	it('asks for the API page cap, because the option list is not paged by the screen', () => {
		expect(OPTION_PAGE_SIZE).toBe(100);
	});
});

describe('withSelectedProvider', () => {
	const options: ProviderOption[] = [{ id: 'openai', name: 'OpenAI' }];

	forEachCase(
		[
			{
				name: 'keeps the list as it is when the address names no provider',
				providerId: '',
				expected: ['openai']
			},
			{
				name: 'keeps the list as it is when the address names one already in it',
				providerId: 'openai',
				expected: ['openai']
			},
			{
				name: 'adds the provider the address names when no read returned it',
				providerId: 'ghost',
				expected: ['openai', 'ghost']
			}
		],
		(testCase) => {
			expect(withSelectedProvider(options, testCase.providerId).map((option) => option.id)).toEqual(
				testCase.expected
			);
		}
	);

	it('does not mutate the list it was given', () => {
		const before = [...options];

		withSelectedProvider(options, 'ghost');

		expect(options).toEqual(before);
	});
});
