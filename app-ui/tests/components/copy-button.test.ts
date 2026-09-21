// Copy control tests (src/lib/components/CopyButton.svelte).
//
// The control has two write paths because the async clipboard API does not exist on the origin this panel
// is usually opened at, and the case that matters is the second one: a live click-through found the copy
// reporting "Copy failed" on an `http://<host>:3000` address, which is where the panel normally runs. So
// each path is driven here, plus the case where neither can write. Both paths are asynchronous, so every
// outcome is waited for rather than read off the tick the click returned on.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import CopyButton from '../../src/lib/components/CopyButton.svelte';

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

afterEach(() => {
	cleanup();
	delete (navigator as { clipboard?: unknown }).clipboard;
	delete (document as { execCommand?: unknown }).execCommand;
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
});
