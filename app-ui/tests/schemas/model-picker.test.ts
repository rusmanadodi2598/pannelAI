// Model picker offer-rule tests (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The rule under test is the one the reference's picker turns on and this panel lacked: only providers
// that are configured right now may be offered, and a provider with no endpoint row cannot route even
// when the registry marks it no_auth (draft 024 §3.7), so it is not offered either. Two further rules keep
// a chip from being an offer that cannot be saved: a connector-only provider and a media model. What the
// search does to the sections has its own file (`model-picker-filter.test.ts`).

import { describe, expect, it } from 'vitest';
import type { CatalogModel } from '$lib/schemas/model';
import { activeProviderIds, pickerSections } from '$lib/schemas/model-picker';
import type { Provider } from '$lib/schemas/provider';

function provider(overrides: Partial<Provider> = {}): Provider {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'apikey',
		auth_type: 'bearer',
		auth_modes: ['api_key'],
		has_oauth: false,
		no_auth: false,
		routability: 'native',
		endpoint_count: 1,
		status_summary: { total: 1, active: 1, disabled: 0, error: 0, rate_limited: 0 },
		...overrides
	};
}

function model(overrides: Partial<CatalogModel> = {}): CatalogModel {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		capabilities: [],
		source: 'registry',
		...overrides
	};
}

describe('activeProviderIds', () => {
	it('keeps only providers with at least one endpoint', () => {
		const active = activeProviderIds([
			provider({ id: 'openai', endpoint_count: 3 }),
			provider({ id: 'anthropic', endpoint_count: 0 }),
			provider({ id: 'oczen', endpoint_count: 0 })
		]);

		expect([...active]).toEqual(['openai']);
	});

	it('drops a no_auth provider with no endpoint, because this gateway still needs a row to route', () => {
		const active = activeProviderIds([
			provider({ id: 'opencode', no_auth: true, endpoint_count: 0 })
		]);

		expect(active.size).toBe(0);
	});
});

describe('pickerSections', () => {
	it('puts the combos first, as their own section', () => {
		const sections = pickerSections({
			catalog: [model()],
			providers: [provider()],
			combos: ['daily']
		});

		expect(sections[0]).toEqual({
			key: 'combos',
			label: 'Combos',
			options: [{ value: 'daily', label: 'daily' }]
		});
	});

	it('offers only the active providers, named by the provider row', () => {
		const sections = pickerSections({
			catalog: [
				model({ id: 'openai/gpt-4o', provider_id: 'openai', model_id: 'gpt-4o' }),
				model({
					id: 'oczen/mimo-v2.6-flash-free',
					provider_id: 'oczen',
					model_id: 'mimo-v2.6-flash-free',
					display_name: 'MiMo v2.6'
				})
			],
			providers: [
				provider({ id: 'openai', name: 'OpenAI', endpoint_count: 1 }),
				provider({ id: 'oczen', name: 'OpenCode Zen Free', endpoint_count: 0 })
			],
			combos: []
		});

		expect(sections).toHaveLength(1);
		expect(sections[0].key).toBe('openai');
		expect(sections[0].label).toBe('OpenAI');
		expect(sections[0].options).toEqual([{ value: 'openai/gpt-4o', label: 'GPT-4o' }]);
	});

	it('drops a catalog row whose provider the list does not carry', () => {
		const sections = pickerSections({
			catalog: [model({ id: 'hidden/secret', provider_id: 'hidden', model_id: 'secret' })],
			providers: [],
			combos: []
		});

		expect(sections).toEqual([]);
	});

	it('sorts one provider rows by label and falls back to the model id when there is no display name', () => {
		const sections = pickerSections({
			catalog: [
				model({ id: 'openai/zeta', provider_id: 'openai', model_id: 'zeta', display_name: 'Zeta' }),
				model({ id: 'openai/alpha', provider_id: 'openai', model_id: 'alpha', display_name: '' }),
				model({ id: 'openai/beta', provider_id: 'openai', model_id: 'beta', display_name: 'Beta' })
			],
			providers: [provider()],
			combos: []
		});

		expect(sections[0].options).toEqual([
			{ value: 'openai/alpha', label: 'alpha' },
			{ value: 'openai/beta', label: 'Beta' },
			{ value: 'openai/zeta', label: 'Zeta' }
		]);
	});

	it('gives a connected provider with no rows a single placeholder option', () => {
		const sections = pickerSections({
			catalog: [],
			providers: [provider({ id: 'th-1', name: 'TH HARBOR 1' })],
			combos: []
		});

		expect(sections[0]).toEqual({
			key: 'th-1',
			label: 'TH HARBOR 1',
			options: [{ value: 'th-1/model-id', label: 'th-1/model-id', placeholder: true }]
		});
	});

	it('keeps the provider order the list route returned', () => {
		const sections = pickerSections({
			catalog: [],
			providers: [
				provider({ id: 'claude', name: 'Claude', endpoint_count: 2 }),
				provider({ id: 'openai', name: 'OpenAI', endpoint_count: 1 })
			],
			combos: []
		});

		expect(sections.map((section) => section.key)).toEqual(['claude', 'openai']);
	});

	it('drops a connector-only provider even when it has an endpoint', () => {
		// The write path refuses its refs (`PROVIDER_NOT_ROUTABLE`, draft 024 F4), so the picker must not
		// offer them: a chip that cannot be saved is the same defect as a chip that cannot route.
		const sections = pickerSections({
			catalog: [
				model({ id: 'commandcode/cc-mini', provider_id: 'commandcode', model_id: 'cc-mini' })
			],
			providers: [provider({ id: 'commandcode', name: 'CommandCode', routability: 'connector' })],
			combos: []
		});

		expect(sections).toEqual([]);
	});

	it('drops a media row and keeps the chat rows of the same provider', () => {
		const sections = pickerSections({
			catalog: [
				model({
					id: 'openai/gpt-4o',
					provider_id: 'openai',
					model_id: 'gpt-4o',
					display_name: 'GPT-4o'
				}),
				model({
					id: 'openai/dall-e-4',
					provider_id: 'openai',
					model_id: 'dall-e-4',
					display_name: 'DALL-E 4',
					kind: 'image'
				})
			],
			providers: [provider()],
			combos: []
		});

		expect(sections[0].options).toEqual([{ value: 'openai/gpt-4o', label: 'GPT-4o' }]);
	});

	it('offers no placeholder under a capability filter, where no cap was reported for one', () => {
		// The vision tab's own call: the catalog was read for `vision`, so the provider that answered with
		// no such model is left out rather than offered as a dashed ref. The combos are not passed either,
		// which is the reference's own rule for a capability-filtered picker
		// (`ModelSelectModal.js:427`).
		const sections = pickerSections({
			catalog: [model({ id: 'openai/gpt-4o', provider_id: 'openai', capabilities: ['vision'] })],
			providers: [
				provider({ id: 'openai', name: 'OpenAI', endpoint_count: 1 }),
				provider({ id: 'th-1', name: 'TH HARBOR 1', endpoint_count: 3 })
			],
			combos: [],
			placeholders: false
		});

		expect(sections.map((section) => section.key)).toEqual(['openai']);
		expect(sections[0].options).toEqual([{ value: 'openai/gpt-4o', label: 'GPT-4o' }]);
	});
});
