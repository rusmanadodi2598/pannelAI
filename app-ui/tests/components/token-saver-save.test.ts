// Token Saver save tests (docs/SPEC-UI/001-SPEC-UI.md §6.7, §7.6).
//
// The rules a reader cannot check by looking at the screen: a save is a whole-document PUT whose untouched
// groups are written back exactly as they were read, a filter the panel cannot offer survives the save,
// and an invalid value in one group does not block another. The assertions are on the request bodies the
// panel produced, because that is where those rules live.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import TokenSaverPage from '../../src/routes/token-saver/+page.svelte';
import { checked, stubTokenSaver, text, tokenSaverDocument } from '../support/token-saver-stub';

async function loaded(): Promise<void> {
	await screen.findByRole('heading', { name: 'RTK' });
}

describe('TokenSaverPage saves', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('saves RTK as a whole document, writing the other groups back as they were read', async () => {
		const stub = stubTokenSaver({
			config: tokenSaverDocument({
				headroom: { enabled: true, url: 'http://localhost:8787', compress_user_messages: true },
				ponytail: { enabled: true, level: 'ultra' }
			})
		});
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));

		await waitFor(() => expect(stub.puts.length).toBe(1));

		expect(stub.puts[0]).toEqual({
			rtk: { enabled: true, filters: [] },
			headroom: { enabled: true, url: 'http://localhost:8787', compress_user_messages: true },
			ponytail: { enabled: true, level: 'ultra' }
		});
	});

	it('keeps a stored filter the panel does not offer, in canonical order', async () => {
		const stub = stubTokenSaver({
			config: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep', 'future-filter'] } })
		});
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /^ls/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));

		await waitFor(() => expect(stub.puts.length).toBe(1));
		expect(stub.puts[0].rtk).toEqual({ enabled: true, filters: ['grep', 'ls', 'future-filter'] });
	});

	it('saves a level change through the Ponytail section', async () => {
		const stub = stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.change(screen.getByLabelText('Level'), { target: { value: 'ultra' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save Ponytail' }));

		await waitFor(() => expect(stub.puts.length).toBe(1));
		expect(stub.puts[0].ponytail).toEqual({ enabled: false, level: 'ultra' });
	});

	it('saves a Headroom URL through the Headroom section alone', async () => {
		const stub = stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.input(screen.getByLabelText('Headroom URL'), {
			target: { value: 'http://localhost:8787' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save Headroom' }));

		await waitFor(() => expect(stub.puts.length).toBe(1));
		expect(stub.puts[0].headroom).toEqual({
			enabled: false,
			url: 'http://localhost:8787',
			compress_user_messages: false
		});
	});

	it('refuses a Headroom save whose URL has no scheme, without sending a request', async () => {
		const stub = stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.input(screen.getByLabelText('Headroom URL'), {
			target: { value: 'localhost:8787' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save Headroom' }));

		expect(text(await screen.findByRole('alert'))).toContain('Use an http or https URL.');
		expect(stub.puts).toEqual([]);
	});

	it('does not let an invalid Headroom URL block an RTK save', async () => {
		const stub = stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.input(screen.getByLabelText('Headroom URL'), {
			target: { value: 'localhost:8787' }
		});
		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));

		await waitFor(() => expect(stub.puts.length).toBe(1));
		expect(stub.puts[0].rtk).toEqual({ enabled: true, filters: [] });
		expect(stub.puts[0].headroom).toEqual({
			enabled: false,
			url: '',
			compress_user_messages: false
		});
	});

	it('marks a section as unsaved until it is saved, then says so', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		expect(screen.getAllByText('Unsaved changes').length).toBe(1);

		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));

		await waitFor(() => expect(screen.queryByText('Unsaved changes')).toBeNull());
		expect(screen.getByText('Saved.')).toBeTruthy();
	});

	it('discards an unsaved change back to what was read', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }));

		expect(checked(screen.getByRole('checkbox', { name: /Compress tool results/ }))).toBe(false);
		expect(screen.queryByText('Unsaved changes')).toBeNull();
	});

	it('warns that an enabled Headroom with no URL will skip every call', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress through Headroom/ }));

		expect(
			screen.getByText(/No URL is set, so every compression call will be skipped/)
		).toBeTruthy();
	});

	it('shows the API refusal as an alert and keeps the form usable', async () => {
		stubTokenSaver({ writeStatus: 400 });
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));

		expect(text(await screen.findByRole('alert'))).toContain('The gateway refused this value.');
		expect(screen.getByRole('button', { name: 'Save RTK' })).toBeTruthy();
	});
});
