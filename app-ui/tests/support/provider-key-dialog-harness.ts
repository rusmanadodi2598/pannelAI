// The entry path the two provider key-dialog test files share
// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// The dialog's modes are driven through the page rather than mounted on their own, because what has to hold
// is the path from this screen to the wire and back into the table below it. Both files enter it the same
// way: open the tab, press the add control, and reach the mode they need.

import { fireEvent, render, screen } from '@testing-library/svelte';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { openAllSections } from './provider-sections';

/**
 * Renders the provider detail screen and opens the tab a key control lives under, because the sections
 * fold closed on mount. The provider id is a parameter because the service-account cases read the
 * vertex provider, while the API key cases read the default one.
 */
export async function renderProvider(id = 'openai'): Promise<void> {
	render(ProviderDetailPage, { props: { params: { provider_id: id }, data: {} } });
	await openAllSections('Connections');
}

/** Types into a field by its label, through the event Svelte 5's `bind:value` listens for. */
export async function fill(label: string, text: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText(label), { target: { value: text } });
}

export async function openDialog(): Promise<void> {
	await fireEvent.click(await screen.findByRole('button', { name: 'Add API Key' }));
	await screen.findByLabelText('Name');
}

export async function openPaste(): Promise<void> {
	await openDialog();
	await fireEvent.click(screen.getByRole('tab', { name: 'Bulk Add' }));
	await screen.findByLabelText('Keys');
}
