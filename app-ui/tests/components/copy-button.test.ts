// Copy control tests (src/lib/components/CopyButton.svelte).
//
// The control has two write paths because the async clipboard API does not exist on the origin this panel
// is usually opened at, and the case that matters is the second one: a live click-through found the copy
// reporting "Copy failed" on an `http://<host>:3000` address, which is where the panel normally runs. So
// each path is driven here, plus the case where neither can write. Both paths are asynchronous, so every
// outcome is waited for rather than read off the tick the click returned on.
//
// The selection path is also driven inside a modal dialog, which is where the control is used on the
// Endpoint & Key screen. jsdom cannot reproduce inertness, so the two facts a browser proved are pinned
// directly: the scratch field's host element, and the answer when that field cannot take focus.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import CopyButton from '../../src/lib/components/CopyButton.svelte';
import { expectIconOnly } from '../support/icon-only';

const VALUE = 'http://127.0.0.1:9090/api/v1';
const COPIED = 'Copied.';
const FAILED = 'Copy failed. Select the text and copy it.';

function stubClipboard(writeText: (value: string) => Promise<void>): void {
	Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
}

/** The selection path, which is what a browser without the async API has left. */
function stubExecCommand(implementation: () => boolean): ReturnType<typeof vi.fn> {
	const spy = vi.fn(implementation);
	document.execCommand = spy as unknown as typeof document.execCommand;
	return spy;
}

async function click(): Promise<void> {
	render(CopyButton, { props: { value: VALUE } });
	await fireEvent.click(screen.getByRole('button', { name: 'Copy' }));
}

/** Renders the control inside a modal dialog, the shape the one-time key modal gives it. */
async function clickInsideDialog(): Promise<void> {
	const dialog = document.createElement('dialog');
	dialog.setAttribute('open', '');
	document.body.appendChild(dialog);
	render(CopyButton, { props: { value: VALUE } }, { baseElement: dialog });
	await fireEvent.click(screen.getByRole('button', { name: 'Copy' }));
}

afterEach(() => {
	cleanup();
	delete (navigator as { clipboard?: unknown }).clipboard;
	delete (document as { execCommand?: unknown }).execCommand;
	document.querySelectorAll('dialog').forEach((dialog) => dialog.remove());
});

describe('CopyButton', () => {
	it('writes through the async clipboard API when the context has one', async () => {
		const copied: string[] = [];
		stubClipboard(async (value) => {
			copied.push(value);
		});

		await click();

		expect(copied).toEqual([VALUE]);
		expect(await screen.findByText(COPIED)).toBeTruthy();
	});

	it('falls back to the selection path when the clipboard API is absent', async () => {
		// The live case: no secure context, so `navigator.clipboard` is undefined rather than refusing.
		const spy = stubExecCommand(() => true);

		await click();

		expect(spy).toHaveBeenCalledWith('copy');
		expect(await screen.findByText(COPIED)).toBeTruthy();
	});

	it('falls back when the async API is present but refuses the write', async () => {
		stubClipboard(async () => Promise.reject(new Error('denied')));
		const spy = stubExecCommand(() => true);

		await click();

		expect(spy).toHaveBeenCalledWith('copy');
		expect(await screen.findByText(COPIED)).toBeTruthy();
	});

	it('reports a failure only when both paths fail', async () => {
		stubExecCommand(() => false);

		await click();

		expect(await screen.findByText(FAILED)).toBeTruthy();
		expect(screen.queryByText(COPIED)).toBeNull();
	});

	it('writes its scratch field inside the open dialog, where the body is inert', async () => {
		// A browser measured this: with a modal dialog open, a field appended to the body cannot take
		// focus or hold a selection, so the copy leaves the clipboard empty. `execCommand` still answers
		// true there, which is why the host element is asserted rather than the command's answer.
		const hosts: (string | undefined)[] = [];
		stubExecCommand(() => {
			hosts.push(document.activeElement?.parentElement?.tagName);
			return true;
		});

		await clickInsideDialog();

		expect(hosts).toEqual(['DIALOG']);
		expect(await screen.findByText(COPIED)).toBeTruthy();
	});

	it('reports a failure when the scratch field cannot take focus', async () => {
		// What inertness does to the field, isolated: focus is refused, so there is no selection to copy
		// and the command's own answer cannot be trusted. The one-time key modal stays gated on this.
		const spy = stubExecCommand(() => true);
		vi.spyOn(HTMLTextAreaElement.prototype, 'focus').mockImplementation(() => {});

		await click();

		expect(await screen.findByText(FAILED)).toBeTruthy();
		expect(spy).not.toHaveBeenCalled();
	});
});

// The icon-only shape the two modals use (owner directive, 2026-09-24). The word leaves the face of the
// button and becomes its accessible name, so the control still announces itself while the value and the
// outcome sentence stay the only text beside it. The outcome is part of the shape: a copy that reported
// nothing would be a dead control, pressed in either shape.
describe('CopyButton, icon-only', () => {
	it('carries no visible text and keeps the name on the button', () => {
		render(CopyButton, { props: { value: VALUE, iconOnly: true } });

		expectIconOnly(screen.getByRole('button', { name: 'Copy' }), 'Copy');
	});

	it('takes a caller label as the accessible name, so the shape is not hard-wired to one word', () => {
		render(CopyButton, { props: { value: VALUE, label: 'Copy install line', iconOnly: true } });

		expectIconOnly(screen.getByRole('button', { name: 'Copy install line' }), 'Copy install line');
	});

	it('reports the outcome the same way, with the value still on screen', async () => {
		const spy = stubExecCommand(() => true);
		render(CopyButton, { props: { value: VALUE, iconOnly: true } });

		await fireEvent.click(screen.getByRole('button', { name: 'Copy' }));

		expect(spy).toHaveBeenCalledWith('copy');
		expect(await screen.findByText(COPIED)).toBeTruthy();
	});
});
