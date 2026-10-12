// The auth_hint sentence on a provider's Connections section (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The registry's auth_hint is the only place the operator learns what credential a provider wants
// before any connection exists: for Qoder it names the PAT prefix and the page that mints one. The
// assertions here hold the two shapes the API answers: the verbatim sentence when the registry
// declares one, and nothing at all when it does not (Go's omitempty delivers an absent key, and a
// provider without a hint must not render an empty paragraph).

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { stubModels } from '../support/model-stub';
import { openAllSections } from '../support/provider-sections';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

async function renderProvider(tab = 'Connections'): Promise<void> {
	render(ProviderDetailPage, { props: { params: { provider_id: 'qoder' }, data: {} } });
	await openAllSections(tab);
}

// The registry's own hint text, byte-for-byte, Vietnamese preposition included (registry.yaml:687,
// a known oddity the generator copied verbatim from the reference).
const QODER_HINT = 'Personal Access Token (pt-...) từ https://qoder.com/account/integrations';

describe('the auth_hint sentence on a provider', () => {
	it('renders the registry hint verbatim above the connections list', async () => {
		stubModels({ authHint: QODER_HINT });
		await renderProvider();

		const sentence = await screen.findByText(QODER_HINT);
		expect(sentence).toBeTruthy();
	});

	it('renders nothing when the provider declares no hint', async () => {
		stubModels({});
		await renderProvider('Connections');

		// Wait for the section itself to land, so the absence assertion reads a loaded screen rather
		// than a still-hydrating one.
		await screen.findByRole('heading', { name: 'Connections' });
		expect(screen.queryByText(QODER_HINT)).toBeNull();
	});
});
