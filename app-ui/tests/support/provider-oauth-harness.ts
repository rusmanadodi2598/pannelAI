// Harness for the provider detail OAuth section tests (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The section is reached through the page, on the Connections tab, with every fold opened: a closed fold
// renders no body, so a test that reaches for the accounts table has to press the heading first. Three files
// read this section (when it appears, the device round, and what a refresh and a callback return), and all
// three enter it the same way.

import { render, screen, waitFor } from '@testing-library/svelte';
import { expect } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { openAllSections } from './provider-sections';

export async function renderProvider(tab = 'Connections'): Promise<void> {
	render(ProviderDetailPage, { props: { params: { provider_id: 'xai' }, data: {} } });
	await openAllSections(tab);
}

export async function waitForAccounts(): Promise<void> {
	await waitFor(() =>
		expect(screen.getByRole('table', { name: /connected oauth accounts/i })).toBeTruthy()
	);
}

export function accountsTable(): HTMLElement {
	return screen.getByRole('table', { name: /connected oauth accounts/i });
}
