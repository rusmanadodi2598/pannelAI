// Provider P0 touch targets, Fase A (F1 copy control, F3 text buttons).
//
// F1: the CopyButton rendered in both model tables must offer a touch-sized
// target, so both tables pass `touchTarget`: the labeled shape answers with
// `min-h-11` and the icon-only shape with `size-11`.
// F3: every retry and clear text button on the provider screens keeps its
// underline identity but must be a 44px row: `inline-flex min-h-11
// items-center underline`, so a thumb can land on it (antislop R-03).
//
// Table-driven per docs/RULLES/TDD.md §2.5: each case names one control on one
// screen, and every case in a table asserts the same class contract, so a fix
// that hardcodes one screen cannot pass the others.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, vi } from 'vitest';
import CopyButton from '../../src/lib/components/CopyButton.svelte';
import ModelCatalogList from '../../src/lib/components/ModelCatalogList.svelte';
import CustomProviderSection from '../../src/lib/components/CustomProviderSection.svelte';
import ProviderCustomModels from '../../src/lib/components/ProviderCustomModels.svelte';
import ProviderEndpoints from '../../src/lib/components/ProviderEndpoints.svelte';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { ApiError } from '../../src/lib/api/errors';
import { expectIconOnly } from '../support/icon-only';
import { createModelDisabledStore } from '../../src/lib/stores/model-disabled.svelte';
import { createModelTestStore } from '../../src/lib/stores/model-test.svelte';
import { createProviderThinkingStore } from '../../src/lib/stores/provider-thinking.svelte';
import { endpointRow, stubEndpoints } from '../support/endpoint-stub';
import { customRow, stubModels } from '../support/model-stub';
import { nodeRow, stubProviderNodes } from '../support/provider-node-stub';
import { renderCard } from '../support/custom-provider-card-harness';
import { forEachCase } from '../support/tables';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

// The page-level error state needs a provider read that fails. The shared
// model stub always answers the provider read, so `getProvider` is replaced
// here while every other route keeps its real implementation.
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

const NODE_ID = 'openai-compatible-01J';
const NODE_PREFIX = 'mycorp';

function catalogRow(): Record<string, unknown> {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		kind: 'chat',
		capabilities: ['vision', 'tools'],
		source: 'registry'
	};
}

/** Serves the model catalog read the way `model-catalog.test.ts` does. */
function stubCatalog(data: Record<string, unknown>[], status = 200): void {
	vi.stubGlobal(
		'fetch',
		async () =>
			new Response(JSON.stringify({ data }), {
				status,
				headers: { 'content-type': 'application/json' }
			})
	);
}

function renderCatalogList(data: Record<string, unknown>[], status = 200): void {
	stubCatalog(data, status);
	render(ModelCatalogList, {
		props: {
			providerId: 'openai',
			disabled: createModelDisabledStore(),
			thinking: createProviderThinkingStore(),
			tests: createModelTestStore(),
			onchanged: () => {}
		}
	});
}

/** Applies the search box the way a keyboard user would. */
async function search(container: HTMLElement, term: string): Promise<void> {
	await fireEvent.input(within(container).getByLabelText('Search models'), {
		target: { value: term }
	});
	await fireEvent.submit(container.querySelector('form') as HTMLFormElement);
}

function expectTouchTextButton(element: HTMLElement, name: string): void {
	const classes = element.className.split(/\s+/);
	for (const expected of ['inline-flex', 'min-h-11', 'items-center', 'underline']) {
		expect(classes, `${name} is missing ${expected}`).toContain(expected);
	}
}

describe('F1: the copy control in both model tables', () => {
	forEachCase(
		[
			{ name: 'catalog row copy button', kind: 'catalog' },
			{ name: 'custom node row copy button', kind: 'custom' },
			{ name: 'icon-only copy button', kind: 'iconOnly' },
			{ name: 'labeled copy button', kind: 'labeled' }
		] as const,
		async ({ kind }) => {
			if (kind === 'catalog') {
				stubCatalog([catalogRow()]);
				render(ModelCatalogList, {
					props: {
						providerId: 'openai',
						disabled: createModelDisabledStore(),
						thinking: createProviderThinkingStore(),
						tests: createModelTestStore(),
						onchanged: () => {}
					}
				});
				const table = await screen.findByRole('table', { name: /model catalog/i });
				const copy = within(table).getByRole('button', { name: 'Copy model address' });
				expect(copy.className).toContain('size-11');
				expectIconOnly(copy, 'Copy model address');
				// The copy lives in the Actions cell beside Disable, not in the Model cell.
				expect(copy.closest('td')?.getAttribute('data-label')).toBe('Actions');
				return;
			}

			if (kind === 'custom') {
				stubModels({
					providers: ['openai', NODE_ID],
					endpoints: [endpointRow({ provider_id: NODE_ID })],
					custom: [customRow({ provider_id: NODE_ID })]
				});
				render(ProviderCustomModels, {
					props: {
						providerId: NODE_ID,
						thinking: createProviderThinkingStore(),
						tests: createModelTestStore(),
						nodePrefix: NODE_PREFIX,
						onchanged: () => {}
					}
				});
				const table = await screen.findByRole('table', { name: /custom models declared/i });
				const copy = within(table).getByRole('button', { name: 'Copy model address' });
				expect(copy.className).toContain('size-11');
				expectIconOnly(copy, 'Copy model address');
				// The copy lives in the Actions cell beside Remove, not in the Model cell.
				expect(copy.closest('td')?.getAttribute('data-label')).toBe('Actions');
				return;
			}

			if (kind === 'iconOnly') {
				render(CopyButton, {
					props: { value: 'openai/gpt-4o', iconOnly: true, touchTarget: true }
				});
				expect(screen.getByRole('button', { name: 'Copy' }).className).toContain('size-11');
				return;
			}

			render(CopyButton, { props: { value: 'openai/gpt-4o', touchTarget: true } });
			expect(screen.getByRole('button', { name: 'Copy' }).className).toContain('min-h-11');
		}
	);
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
				const { container } = render(ModelCatalogList, {
					props: {
						providerId: 'openai',
						disabled: createModelDisabledStore(),
						thinking: createProviderThinkingStore(),
						tests: createModelTestStore(),
						onchanged: () => {}
					}
				});
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
				const { container } = render(ModelCatalogList, {
					props: {
						providerId: 'openai',
						disabled: createModelDisabledStore(),
						thinking: createProviderThinkingStore(),
						tests: createModelTestStore(),
						onchanged: () => {}
					}
				});
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
