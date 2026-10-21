// F1: the copy control in both model tables (docs/SPEC-UI/001-SPEC-UI.md §8.7, §11.3).
//
// The copy control must offer a touch-sized target: the icon-only shape answers with `size-11` and the
// labeled one with `min-h-11`. It lives in the Actions cell beside the row's other control, which is also
// asserted here, because a copy sitting in the Model cell would read as part of the value it copies.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, vi } from 'vitest';
import CopyButton from '../../src/lib/components/CopyButton.svelte';
import ModelCatalogList from '../../src/lib/components/ModelCatalogList.svelte';
import ProviderCustomModels from '../../src/lib/components/ProviderCustomModels.svelte';
import { expectIconOnly } from '../support/icon-only';
import { createModelDisabledStore } from '../../src/lib/stores/model-disabled.svelte';
import { createModelTestStore } from '../../src/lib/stores/model-test.svelte';
import { createProviderThinkingStore } from '../../src/lib/stores/provider-thinking.svelte';
import { endpointRow } from '../support/endpoint-stub';
import { customRow, stubModels } from '../support/model-stub';
import { forEachCase } from '../support/tables';
import { catalogRow, NODE_ID, NODE_PREFIX, stubCatalog } from '../support/provider-touch-harness';

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
