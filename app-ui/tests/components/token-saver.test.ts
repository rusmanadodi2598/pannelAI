// Token Saver screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.7, §8.3).
//
// This file covers what the screen shows and reads: the three sections in the order §6.7 fixes, the
// stored values in the controls, the allowlist copy that keeps "every filter" apart from "no filter", and
// the two states a load can end in. The save behaviors live in `token-saver-save.test.ts`.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import TokenSaverPage from '../../src/routes/token-saver/+page.svelte';
import { checked, value } from '../support/dom';
import { stubTokenSaver, tokenSaverDocument } from '../support/token-saver-stub';

async function loaded(): Promise<void> {
	await screen.findByRole('heading', { name: 'RTK' });
}

function headingOrder(): string[] {
	return screen
		.getAllByRole('heading', { level: 2 })
		.map((heading) => heading.textContent?.trim() ?? '');
}

describe('TokenSaverPage', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders the three sections in the order §6.7 fixes, then the two notes', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		expect(headingOrder()).toEqual([
			'RTK',
			'Headroom',
			'Ponytail',
			'Native engine',
			'Skip the savers for one request'
		]);
	});

	it('reads the stored configuration into the controls', async () => {
		stubTokenSaver({
			config: tokenSaverDocument({
				rtk: { enabled: true, filters: ['grep'] },
				headroom: { enabled: true, url: 'http://localhost:8787', compress_user_messages: true },
				ponytail: { enabled: true, level: 'ultra' }
			})
		});
		render(TokenSaverPage);
		await loaded();

		expect(checked(screen.getByRole('checkbox', { name: /Compress tool results/ }))).toBe(true);
		expect(checked(screen.getByRole('checkbox', { name: /^grep/ }))).toBe(true);
		expect(checked(screen.getByRole('checkbox', { name: /^git-diff/ }))).toBe(false);
		expect(value(screen.getByLabelText('Headroom URL'))).toBe('http://localhost:8787');
		expect(value(screen.getByLabelText('Level'))).toBe('ultra');
	});

	it('reads an empty allowlist as every filter, not as none', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		expect(screen.getByText('Every filter is eligible.')).toBeTruthy();
	});

	it('counts a partial allowlist against the twelve', async () => {
		stubTokenSaver({
			config: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep', 'ls'] } })
		});
		render(TokenSaverPage);
		await loaded();

		expect(screen.getByText('2 of 12 filters allowed.')).toBeTruthy();
	});

	it('names a stored filter the panel cannot offer, and says it is kept', async () => {
		stubTokenSaver({
			config: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep', 'future-filter'] } })
		});
		render(TokenSaverPage);
		await loaded();

		expect(screen.getByText(/Stored but not offered here: future-filter/)).toBeTruthy();
	});

	it('states that the native engine is planned rather than shipping a disabled control', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		expect(screen.getByText('Planned')).toBeTruthy();
		expect(screen.getByText(/specified in 002-TOKEN-SAVER/)).toBeTruthy();
	});

	it('offers the per-request bypass header as copyable text', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		expect(screen.getByText('X-Token-Saver: off')).toBeTruthy();
		expect(screen.getByRole('button', { name: /Copy header/ })).toBeTruthy();
	});

	it('offers a retry when the configuration cannot be read', async () => {
		stubTokenSaver({ readStatus: 500 });
		render(TokenSaverPage);

		expect(
			await screen.findByText('The token saver configuration could not be loaded')
		).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy();
	});

	it('renders no caveman control anywhere', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		expect(screen.queryByText(/caveman/i)).toBeNull();
	});
});
