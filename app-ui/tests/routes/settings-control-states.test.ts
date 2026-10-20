// Every control the Settings screen renders carries a pressed colour, not only a hover one (DESIGN.md §3.5).
//
// The screen is five tabs and three kinds of control: the write that ends each editor tab, the discard that
// appears beside it once the draft moves, the pointer tabs' link off the screen, the dialog that confirms
// turning the password requirement off, and the retry a failed load leaves behind. All of them were written
// out at the call site, and none of them painted anything under a press; on a touch layout the hover rule
// they did carry is switched off, so the whole screen answered nothing.
//
// The tabs are opened one at a time because that is how the screen renders them, and each pass walks every
// control the opened tab put on the page rather than a hand-picked list.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SettingsPage from '../../src/routes/settings/+page.svelte';
import { openTab, stubSettings } from '../support/settings-stub';
import { visit } from '../support/page.svelte';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button'));
}

describe('Settings control states', () => {
	beforeEach(() => {
		visit('/settings');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control of the Routing tab a pressed colour', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		const seen = controls();
		// Five tab buttons and the write that ends the form.
		expect(seen.length).toBeGreaterThanOrEqual(6);
		expectEveryControlHasStates(seen, 'the Settings routing tab');
	});

	it('keeps the save control on its accent while the write is in flight', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		const save = screen.getByRole('button', { name: 'Save routing settings' });
		expect(save.getAttribute('aria-busy')).not.toBe('true');

		// The primitive's own rule: a control greyed out mid-call reads as a broken button, so a write in
		// flight keeps the accent and says so with aria-busy instead.
		expect(save.className).toContain("[&[aria-busy='true']:disabled]:bg-[var(--color-accent)]");
		expect(save.className).toContain('hover:bg-[var(--color-accent-hover)]');
		expect(save.className).toContain('active:bg-[var(--color-accent-press)]');
	});

	it('gives the discard action its states the moment the draft moves', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Routing');

		await fireEvent.input(screen.getByLabelText('Routing sticky limit'), {
			target: { value: '5' }
		});

		expectControlStates(await screen.findByRole('button', { name: 'Discard changes' }));
	});

	it('gives the security dialog its two answers', async () => {
		stubSettings();
		render(SettingsPage);
		await openTab('Security');

		expectEveryControlHasStates(controls(), 'the Settings security tab');

		// The checkbox's label carries its warning sentence too, so the control is found by the demand it
		// opens with rather than by the whole block of text.
		await fireEvent.change(screen.getByLabelText(/Require a password to open the panel/), {
			target: { checked: false }
		});

		expectControlStates(await screen.findByRole('button', { name: 'Turn it off' }));
		expectControlStates(await screen.findByRole('button', { name: 'Keep the password' }));
	});

	it('gives the pointer tab its link the same states as a button', async () => {
		// A standalone action link is pressed as often as a button is, and an underline with no colour
		// behind it answers nothing on a touch screen. The primitive renders an anchor, so the link keeps
		// its role and takes the scale.
		stubSettings();
		render(SettingsPage);
		await openTab('Network');

		const link = screen.getByRole('link', { name: 'Open Proxy Pools' });
		expectControlStates(link);
		expect(link.getAttribute('href')).toBe('/proxy-pools');
	});

	it('gives a failed load its retry', async () => {
		stubSettings({ readStatus: 500 });
		render(SettingsPage);

		expectControlStates(await screen.findByRole('button', { name: 'Try again' }));
	});
});
