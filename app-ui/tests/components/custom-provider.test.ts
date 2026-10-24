// The Custom provider add controls (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The registry table is the one list of providers now, including synthesized custom nodes. This section
// only creates one, so it carries the two type buttons and the sentence that says where the new row will
// appear. The dialog and its write paths are in `custom-provider-add.test.ts`.

import { cleanup, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { stubProviderNodes } from '../support/provider-node-stub';
import { renderSection } from '../support/custom-provider-harness';
import { squashed } from '../support/dom';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the custom provider add controls', () => {
	it('offers both compatible types and says the provider lands in the registry', () => {
		renderSection(stubProviderNodes());

		expect(screen.getByRole('heading', { name: 'Add a custom provider' })).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Add Anthropic Compatible' })).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Add OpenAI Compatible' })).toBeTruthy();
		expect(squashed(screen.getByText(/appears in the registry below/))).toContain(
			'appears in the registry below'
		);
	});

	it('renders no second list; the registry table owns the rows', () => {
		renderSection(stubProviderNodes());

		expect(screen.queryByRole('list')).toBeNull();
	});
});
