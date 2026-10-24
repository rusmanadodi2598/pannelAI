// Token Saver editing rules (docs/SPEC-UI/001-SPEC-UI.md §6.7).
//
// The rules here are the ones a screenshot cannot show: a discard reaches one section, a save in one
// section keeps the draft in another, two sections cannot write at once, and a failed write reports beside
// the control that retries it. The failed re-read after a save keeps the form and offers its own retry.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import TokenSaverPage from '../../src/routes/token-saver/+page.svelte';
import { checked, value } from '../support/dom';
import { stubTokenSaver, tokenSaverDocument } from '../support/token-saver-stub';

async function loaded(): Promise<void> {
	await screen.findByRole('heading', { name: 'RTK' });
}

describe('TokenSaverPage editing', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('discards only the section whose control was pressed', async () => {
		stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		const rtk = screen.getByRole('checkbox', { name: /Compress tool results/ });
		const headroom = screen.getByRole('checkbox', { name: /Compress through Headroom/ });
		await fireEvent.click(rtk);
		await fireEvent.click(headroom);
		expect(screen.getAllByText('Unsaved changes')).toHaveLength(2);

		// DOM order is RTK, Headroom, Ponytail, so the first discard belongs to RTK.
		await fireEvent.click(screen.getAllByRole('button', { name: 'Discard changes' })[0]);

		expect(checked(rtk)).toBe(false);
		expect(checked(headroom)).toBe(true);
		expect(screen.getAllByText('Unsaved changes')).toHaveLength(1);
	});

	it('keeps an unsaved group when another section is saved and re-read', async () => {
		const stub = stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		const headroomUrl = screen.getByLabelText('Headroom URL');
		await fireEvent.input(headroomUrl, { target: { value: 'http://localhost:8787' } });
		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));

		await waitFor(() => expect(stub.puts).toHaveLength(1));
		await waitFor(() =>
			expect((screen.getByRole('button', { name: 'Save RTK' }) as HTMLButtonElement).disabled).toBe(
				false
			)
		);

		// The save wrote the server's empty Headroom, but the operator's URL was never discarded.
		expect(stub.puts[0].headroom).toEqual({
			enabled: false,
			url: '',
			compress_user_messages: false
		});
		expect(value(headroomUrl)).toBe('http://localhost:8787');
		expect(screen.getAllByText('Unsaved changes')).toHaveLength(1);
	});

	it('runs one section save at a time and blocks the others while it lands', async () => {
		const stub = stubTokenSaver({ putDelayMs: 120 });
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress through Headroom/ }));

		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));
		await waitFor(() => expect(stub.puts).toHaveLength(1));

		const headroomSave = screen.getByRole('button', {
			name: 'Save Headroom'
		}) as HTMLButtonElement;
		expect(headroomSave.disabled).toBe(true);
		await fireEvent.click(headroomSave);
		expect(stub.puts).toHaveLength(1);

		await waitFor(() => expect(headroomSave.disabled).toBe(false));
		expect(stub.puts).toHaveLength(1);
	});

	it('keeps an edit made while its own section is saving', async () => {
		const stub = stubTokenSaver({ putDelayMs: 120 });
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));
		await waitFor(() => expect(stub.puts).toHaveLength(1));

		const gitDiff = screen.getByRole('checkbox', { name: /^git-diff/ });
		await fireEvent.click(gitDiff);

		await waitFor(() =>
			expect((screen.getByRole('button', { name: 'Save RTK' }) as HTMLButtonElement).disabled).toBe(
				false
			)
		);

		// The filter was not in the PUT, so it stays as an unsaved edit instead of being lost.
		expect(stub.puts[0].rtk).toEqual({ enabled: true, filters: [] });
		expect(checked(gitDiff)).toBe(true);
		expect(screen.getByText('Unsaved changes')).toBeTruthy();
	});

	it('reports a failed Ponytail save inside the Ponytail section', async () => {
		stubTokenSaver({ writeStatus: 400 });
		render(TokenSaverPage);
		await loaded();

		await fireEvent.click(screen.getByRole('checkbox', { name: /Apply the Ponytail bias/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save Ponytail' }));

		const alert = await screen.findByRole('alert');
		const section = alert.closest('section');
		expect(section?.textContent).toContain('Ponytail');
		expect(section?.textContent).toContain('The gateway refused this value.');
	});

	it('keeps the form and offers a retry when the post-save re-read fails', async () => {
		const stub = stubTokenSaver();
		render(TokenSaverPage);
		await loaded();

		stub.readStatus = 500;
		await fireEvent.click(screen.getByRole('checkbox', { name: /Compress tool results/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Save RTK' }));

		const alert = await screen.findByRole('alert');
		expect(alert.textContent).toContain('Could not refresh the token saver settings.');
		expect(alert.textContent).toContain('The form still shows the last configuration it received.');
		expect(screen.getByRole('heading', { name: 'RTK' })).toBeTruthy();
		await waitFor(() => expect(document.activeElement).toBe(alert));

		stub.readStatus = 200;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
		expect(screen.getByText('Saved.')).toBeTruthy();
	});

	it('states that each section saves on its own', async () => {
		stubTokenSaver({
			config: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep'] } })
		});
		render(TokenSaverPage);
		await loaded();

		expect(
			screen.getByText('How the gateway trims a request. Each section saves on its own.')
		).toBeTruthy();
	});
});
