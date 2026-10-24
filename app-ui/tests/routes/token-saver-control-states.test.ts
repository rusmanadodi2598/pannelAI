// Every control the Token Saver screen renders carries a pressed colour (DESIGN.md §3.5).
//
// Three of the four were written at the call site: the section commit, whose `disabled:opacity-60` is the
// fade this very document records as wrong for a filled control, the discard beside it, and the copy
// control that hovered onto `surface-2`, which is the hovered ground rather than its own.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import TokenSaverPage from '../../src/routes/token-saver/+page.svelte';
import { stubTokenSaver } from '../support/token-saver-stub';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button, a[data-slot="button"]'));
}

async function loaded(): Promise<void> {
	await screen.findByRole('heading', { name: 'RTK' });
}

describe('Token saver control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control on the screen a pressed colour', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		// Three section commits, one discard per dirty section, and the bypass copy.
		const seen = controls();
		expect(seen.length).toBeGreaterThanOrEqual(4);
		expectEveryControlHasStates(seen, 'the Token Saver screen');
	});

	it('puts the section commit on the shared band, in flight included', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		const commit = screen.getByRole('button', { name: 'Save RTK' });
		expectControlStates(commit);

		// The two classes that say this is the band and not a screen's own row: the filled accent, and the
		// rule that keeps the accent while a write is in flight instead of fading both label and fill.
		expect(commit.className).toContain('bg-[var(--color-accent)]');
		expect(commit.className).toContain("[&[aria-busy='true']:disabled]:bg-[var(--color-accent)]");
		expect(commit.className).not.toContain('opacity-60');
	});

	it('makes the unsaved discard a control rather than a bare underline', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));

		// One marker per dirty section: the toggle moved RTK alone, and pressing any discard resets the
		// whole draft, which is why the button belongs to the section but the state does not.
		const discards = screen.getAllByRole('button', { name: 'Discard changes' });
		expect(discards).toHaveLength(1);
		for (const discard of discards) expectControlStates(discard);
	});

	it('gives the bypass header the copy control that can reach a clipboard', async () => {
		// The panel is normally reached on an insecure origin, where `navigator.clipboard` does not exist,
		// so the shared control's selection path is the only one that copies anything here.
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		const copy = screen.getByRole('button', { name: 'Copy header' });
		expectControlStates(copy);
		expect(copy.className).toContain('min-h-11');
	});

	it('gives a failed read its retry', async () => {
		stubTokenSaver({ readStatus: 500 });
		render(TokenSaverPage);

		expectControlStates(await screen.findByRole('button', { name: 'Try again' }));
	});

	it('makes every checkbox label a row a thumb can land on', async () => {
		// The label is the target, not the 16px box inside it: clicking the words toggles the checkbox, so
		// SPEC-UI §8.7's 44px applies to the label. Twelve allowlist rows and three section rows measured
		// 36px before this row existed.
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		const labels = Array.from(document.querySelectorAll('label:has(input[type="checkbox"])'));
		expect(labels).toHaveLength(16);
		for (const label of labels) {
			const name = label.textContent?.trim().slice(0, 24) ?? 'unnamed row';
			expect(label.className, `${name} has no 44px row`).toContain('min-h-11');
		}
	});
});
