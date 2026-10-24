// The add-form page entry the custom-model write tests share (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A custom row joins the catalog, so the write tests drive the provider detail page rather than mounting the
// form alone. Both files enter it the same way and start from the same fixture: one catalog row and two
// custom rows, one of them another provider's.

import { render, screen, waitFor } from '@testing-library/svelte';
import { expect } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { catalogRow, customRow, stubModels, type ModelStub } from './model-stub';
import { openAllSections } from './provider-sections';

export function customModelStub(): ModelStub {
	return stubModels({
		catalog: [catalogRow()],
		custom: [
			customRow(),
			customRow({
				id: 'mdl_02',
				provider_id: 'anthropic',
				model_id: 'claude-3-haiku',
				display_name: 'Claude 3 Haiku'
			})
		]
	});
}

export async function renderProvider(providerId = 'openai'): Promise<void> {
	render(ProviderDetailPage, { props: { params: { provider_id: providerId }, data: {} } });
	await openAllSections('Models');
}

/** Fills the add form. The fields are found by their label, so a renamed field fails here. */
export function fillForm(fields: {
	model_id?: string;
	display_name?: string;
	capabilities?: string;
}): void {
	if (fields.model_id !== undefined) {
		const input = screen.getByLabelText('Model id') as HTMLInputElement;
		input.value = fields.model_id;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
	if (fields.display_name !== undefined) {
		const input = screen.getByLabelText('Display name (optional)') as HTMLInputElement;
		input.value = fields.display_name;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
	if (fields.capabilities !== undefined) {
		const input = screen.getByLabelText('Capabilities (optional)') as HTMLInputElement;
		input.value = fields.capabilities;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
}

export async function waitForCustomTable(): Promise<void> {
	await waitFor(() =>
		expect(screen.getByRole('table', { name: /custom models declared/i })).toBeTruthy()
	);
}
