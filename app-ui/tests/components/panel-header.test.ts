// Panel header tests (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// The header's API base control is one button that opens the dialog, so what is worth asserting is that
// the dialog is unreachable until the control is used, that it is the one the control names, and that the
// explicit close path takes it away again. The dialog's own states are its own test file; the read is
// stubbed here only so the dialog can finish opening.
//
// Escape is not asserted here: it comes from the native <dialog> element the Modal is built on, which jsdom
// does not implement (R-32 is inherited from the shell pass).

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import HeaderHarness from '../support/HeaderHarness.svelte';
import { stubApiBase } from '../support/api-base-stub';

// The sidebar's mobile check reads a media query, which jsdom does not implement. Reporting "not mobile"
// keeps the desktop branch under test.
function stubMatchMedia(): void {
	window.matchMedia = ((query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addEventListener: () => {},
		removeEventListener: () => {},
		addListener: () => {},
		removeListener: () => {},
		dispatchEvent: () => false
	})) as unknown as typeof window.matchMedia;
}

beforeEach(stubMatchMedia);

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('panel header', () => {
	it('keeps the API base dialog closed until its control is used', async () => {
		// The dialog reads the gateway address from the panel server when it opens, so the answer has to
		// come from somewhere before the tabs exist.
		stubApiBase();
		render(HeaderHarness);

		const opener = screen.getByRole('button', { name: 'API base' });
		expect(opener.getAttribute('aria-haspopup')).toBe('dialog');
		expect(screen.queryByRole('tab', { name: 'Base URL' })).toBeNull();

		await fireEvent.click(opener);

		expect(await screen.findByRole('tab', { name: 'Base URL' })).toBeTruthy();
		expect(screen.getByRole('tab', { name: 'cURL' })).toBeTruthy();
		expect(screen.getByRole('tab', { name: 'OpenAI client' })).toBeTruthy();
	});

	it('closes the dialog from its own close control', async () => {
		stubApiBase();
		render(HeaderHarness);
		await fireEvent.click(screen.getByRole('button', { name: 'API base' }));

		await fireEvent.click(screen.getByRole('button', { name: 'Close dialog' }));

		expect(screen.queryByRole('tab', { name: 'Base URL' })).toBeNull();
		expect(screen.getByRole('button', { name: 'API base' })).toBeTruthy();
	});
});
