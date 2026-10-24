// The Changelog screen's controls carry the states (DESIGN.md §3.5).
//
// One control was still written at the call site: the retry a failed release read leaves behind, an underline
// that painted nothing under a press. It is the screen's second action after the refresh, and on the touch
// layout where `hover:` never applies it answered nothing at all.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ChangelogPage from '../../src/routes/changelog/+page.svelte';
import { stubChangelog } from '../support/changelog-stub';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

const FAILED_READ = {
	releases: { error: { code: 'INTERNAL_ERROR', message: 'The gateway is down.' } },
	releasesStatus: 500
};

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('Changelog control states', () => {
	it('gives the failed read its retry, on the ramp and still an underlined action', async () => {
		stubChangelog(FAILED_READ);
		render(ChangelogPage);

		const retry = await screen.findByRole('button', { name: 'Try again' });
		expectControlStates(retry);
		expect(retry.className).toContain('underline');
		expect(retry.className).toContain('min-h-11');
		expect(retry.className).not.toContain('bg-[var(--color-accent)]');
	});

	it('gives every control on the screen a hover and a pressed step', async () => {
		stubChangelog(FAILED_READ);
		render(ChangelogPage);
		await screen.findByRole('alert');

		// The refresh control and the retry, which is every button this screen offers in its failure state.
		expectEveryControlHasStates(
			Array.from(document.querySelectorAll('button')),
			'the Changelog screen'
		);
	});
});
