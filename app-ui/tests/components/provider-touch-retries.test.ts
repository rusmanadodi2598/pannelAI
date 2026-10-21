// F3: every retry, clear and text link on the provider screens is a 44px row
// (docs/SPEC-UI/001-SPEC-UI.md §8.7, §11.3).
//
// These controls keep their underline identity and must still be a row a thumb can land on. The table below
// names one control on one screen per case and asserts the same class contract for all of them, so a fix
// that hardcodes one screen cannot pass the others.

import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, vi } from 'vitest';
import ProviderCustomModels from '../../src/lib/components/ProviderCustomModels.svelte';
import CustomProviderSection from '../../src/lib/components/CustomProviderSection.svelte';
import ProviderEndpoints from '../../src/lib/components/ProviderEndpoints.svelte';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { ApiError } from '../../src/lib/api/errors';
import { createModelTestStore } from '../../src/lib/stores/model-test.svelte';
import { createProviderThinkingStore } from '../../src/lib/stores/provider-thinking.svelte';
import { customRow, stubModels } from '../support/model-stub';
import { stubEndpoints } from '../support/endpoint-stub';
import { nodeRow, stubProviderNodes } from '../support/provider-node-stub';
import { renderCard } from '../support/custom-provider-card-harness';
import { forEachCase } from '../support/tables';
import {
	catalogRow,
	expectTouchTextButton,
	renderCatalogList,
	renderStubbedCatalog,
	search,
	stubCatalog
} from '../support/provider-touch-harness';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

// The page-level cases need a provider read that fails, and the shared model stub always answers it, so
// `getProvider` is replaced here while every other route keeps its real implementation.
vi.mock('$lib/api/providers', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/api/providers')>();
	return {
		...actual,
		getProvider: async () => ({
			ok: false,
			error: new ApiError(500, 'INTERNAL_ERROR', 'The registry entry could not be read.')
		})
	};
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('F3: retry and clear text buttons', () => {
	forEachCase(
		[
			{ name: 'catalog load retry', kind: 'catalog-retry' },
			{ name: 'catalog filter-bar clear', kind: 'catalog-clear-bar' },
			{ name: 'catalog empty-state clear', kind: 'catalog-clear-empty' },
			{ name: 'custom models load retry', kind: 'custom-retry' },
			{ name: 'connections load retry', kind: 'endpoints-retry' },
			{ name: 'custom card load retry', kind: 'card-retry' },
			{ name: 'custom provider section load retry', kind: 'section-retry' },
			{ name: 'page breadcrumb link', kind: 'page-breadcrumb' },
			{ name: 'page load retry', kind: 'page-retry' },
			{ name: 'page back link', kind: 'page-back' }
		] as const,
		async ({ kind, name }) => {
			if (kind === 'catalog-retry') {
				renderCatalogList([], 500);
				const button = await screen.findByRole('button', { name: 'Try again' });
				expectTouchTextButton(button, name);
				return;
			}

			if (kind === 'catalog-clear-bar') {
				stubCatalog([catalogRow()]);
				const container = renderStubbedCatalog();
				await screen.findByText('GPT-4o');
				await search(container, 'gpt-4o');
				await waitFor(() =>
					expect(screen.getByRole('button', { name: 'Clear filters' })).toBeTruthy()
				);
				expectTouchTextButton(screen.getByRole('button', { name: 'Clear filters' }), name);
				return;
			}

			if (kind === 'catalog-clear-empty') {
				stubCatalog([]);
				const container = renderStubbedCatalog();
				await screen.findByText('This provider offers no models');
				await search(container, 'does-not-exist');
				await screen.findByText('No models found for this filter');
				const buttons = screen.getAllByRole('button', { name: 'Clear filters' });
				expect(buttons.length).toBeGreaterThan(0);
				for (const button of buttons) expectTouchTextButton(button, name);
				return;
			}

			if (kind === 'custom-retry') {
				stubModels({ readStatus: 500 });
				render(ProviderCustomModels, {
					props: {
						providerId: 'openai',
						thinking: createProviderThinkingStore(),
						tests: createModelTestStore(),
						onchanged: () => {}
					}
				});
				const button = await screen.findByRole('button', { name: 'Try again' });
				expectTouchTextButton(button, name);
				return;
			}

			if (kind === 'endpoints-retry') {
				stubEndpoints({ listStatus: 500 });
				render(ProviderEndpoints, { props: { providerId: 'openai' } });
				const button = await screen.findByRole('button', { name: 'Try again' });
				expectTouchTextButton(button, name);
				return;
			}

			if (kind === 'section-retry') {
				const onreload = vi.fn(async () => {});
				render(CustomProviderSection, {
					props: {
						nodes: [],
						loading: false,
						error: new ApiError(500, 'INTERNAL_ERROR', 'the list is unavailable'),
						onreload
					}
				});
				const button = await screen.findByRole('button', { name: 'Try again' });
				expectTouchTextButton(button, name);
				return;
			}

			if (kind === 'card-retry') {
				const stub = stubProviderNodes({ nodes: [nodeRow()], readStatus: 500 });
				renderCard(stub);
				const button = await screen.findByRole('button', { name: 'Try again' });
				expectTouchTextButton(button, name);
				return;
			}

			stubModels({ catalog: [catalogRow()], custom: [customRow()] });
			render(ProviderDetailPage, { props: { params: { provider_id: 'openai' }, data: {} } });

			if (kind === 'page-breadcrumb') {
				const link = await screen.findByRole('link', { name: 'Provider' });
				expectTouchTextButton(link, name);
				return;
			}

			if (kind === 'page-retry') {
				const button = await screen.findByRole('button', { name: 'Try again' });
				expectTouchTextButton(button, name);
				return;
			}

			const link = await screen.findByRole('link', { name: 'Back to the registry' });
			expectTouchTextButton(link, name);
		}
	);
});
