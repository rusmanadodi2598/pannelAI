// Media Provider save tests (docs/SPEC-UI/001-SPEC-UI.md §6.8, SPEC-API §7.10).
//
// §6.8 asks for two things a render test cannot check: the form blocks the save it can prove the server
// would refuse, with the server's exact message, and the server still validates. So the assertions here
// are about what reached the wire and what the card shows afterwards, and the stub refuses the same save
// the server refuses, which is what makes "the two agree" a testable claim.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import MediaProvidersPage from '../../src/routes/media-providers/[kind]/+page.svelte';
import { mediaRow, stubMediaProviders } from '../support/media-stub';
import { squashed, value } from '../support/dom';
import { forEachCase } from '../support/tables';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

function renderKind(kind: string): void {
	render(MediaProvidersPage, { props: { params: { kind }, data: {} } });
}

async function loaded(): Promise<HTMLInputElement> {
	const field = await screen.findByLabelText('Base URL');
	return field as HTMLInputElement;
}

async function save(): Promise<void> {
	screen.getByRole('button', { name: 'Save' }).click();
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the save it can prove would fail', () => {
	it('blocks the save and quotes the server when the registry declares no base URL', async () => {
		// The row's own values say the registry has none: the source is `registry` and the value is empty,
		// so an empty field means no base URL at all. The panel knows this without asking.
		const stub = stubMediaProviders({
			rows: [mediaRow({ base_url: '', base_url_source: 'registry' })],
			registry: { openai: { base_url: '', default_model: 'tts-1' } }
		});
		renderKind('tts');

		await loaded();
		await save();

		await waitFor(() =>
			expect(screen.getByText('provider openai has no tts base_url; set one')).toBeTruthy()
		);
		expect(stub.patches, 'the panel asked the server a question it could answer itself').toEqual(
			[]
		);
	});

	it('blocks it when the field holds only whitespace, because the API trims before deciding', async () => {
		const stub = stubMediaProviders({
			rows: [mediaRow({ base_url: '', base_url_source: 'registry' })],
			registry: { openai: { base_url: '', default_model: 'tts-1' } }
		});
		renderKind('tts');

		const field = await loaded();
		await fireEvent.input(field, { target: { value: '   ' } });
		await save();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(stub.patches).toEqual([]);
	});

	it('shows the refusal in the alert role, because it is the answer to the action the operator took', async () => {
		stubMediaProviders({
			rows: [mediaRow({ base_url: '', base_url_source: 'registry' })],
			registry: { openai: { base_url: '', default_model: 'tts-1' } }
		});
		renderKind('tts');

		await loaded();
		await save();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(squashed(screen.getByRole('alert'))).toBe(
			'provider openai has no tts base_url; set one'
		);
	});

	it('saves once the operator fills the field the refusal asked for', async () => {
		const stub = stubMediaProviders({
			rows: [mediaRow({ base_url: '', base_url_source: 'registry' })],
			registry: { openai: { base_url: '', default_model: 'tts-1' } }
		});
		renderKind('tts');

		const field = await loaded();
		await fireEvent.input(field, {
			target: { value: 'http://127.0.0.1:8095/v1/audio/speech' }
		});
		await save();

		await waitFor(() => expect(stub.patches.length).toBe(1));
		expect(stub.patches[0].body.base_url).toBe('http://127.0.0.1:8095/v1/audio/speech');
		expect(screen.queryByRole('alert')).toBeNull();
	});
});

describe('what reaches the wire', () => {
	it('sends the panel kind in the API spelling, which is the whole point of the mapping', async () => {
		// The address says `web` and the body must say `search`. A body carrying the slug would be refused
		// by the API's own closed set, and only a test that renders the web kind can catch it.
		const stub = stubMediaProviders({
			rows: [
				mediaRow({
					kind: 'search',
					provider_id: 'brave-search',
					provider_name: 'Brave Search',
					base_url: 'https://api.search.brave.com/res/v1',
					default_model: ''
				})
			]
		});
		renderKind('web');

		await loaded();
		await save();

		await waitFor(() => expect(stub.patches.length).toBe(1));
		expect(stub.patches[0].id).toBe('brave-search');
		expect(stub.patches[0].body.kind).toBe('search');
	});

	it('sends both fields, so a save cannot silently leave the other one behind', async () => {
		const stub = stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		await loaded();
		await save();

		await waitFor(() => expect(stub.patches.length).toBe(1));
		expect(Object.keys(stub.patches[0].body).sort()).toEqual(['base_url', 'default_model', 'kind']);
	});

	it('sends the model the selector holds, not the model the card was rendered with', async () => {
		const stub = stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		await loaded();
		await fireEvent.change(screen.getByLabelText('Default model'), {
			target: { value: 'tts-1-hd' }
		});
		await save();

		await waitFor(() => expect(stub.patches.length).toBe(1));
		expect(stub.patches[0].body.default_model).toBe('tts-1-hd');
	});

	it('sends an empty base URL over an override, because clearing it is how the override is undone', async () => {
		// The panel cannot prove this save is safe: the registry's value is only read when the override is
		// empty, so it is not on the wire. It sends the clear and lets the server answer.
		const stub = stubMediaProviders({
			rows: [
				mediaRow({ base_url: 'http://127.0.0.1:8095/v1/audio/speech', base_url_source: 'override' })
			],
			registry: {
				openai: { base_url: 'https://api.openai.com/v1/audio/speech', default_model: 'tts-1' }
			}
		});
		renderKind('tts');

		const field = await loaded();
		expect(value(field)).toBe('http://127.0.0.1:8095/v1/audio/speech');

		await fireEvent.input(field, { target: { value: '' } });
		await save();

		await waitFor(() => expect(stub.patches.length).toBe(1));
		expect(stub.patches[0].body.base_url).toBe('');
		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('refuses a value the API could not dial, without asking the server', async () => {
		const stub = stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		const field = await loaded();
		await fireEvent.input(field, { target: { value: 'api.example.com/v1' } });
		await save();

		await waitFor(() => expect(screen.getByText('Use an http or https URL.')).toBeTruthy());
		expect(stub.patches).toEqual([]);
	});
});

describe('what the card shows after a save', () => {
	it('re-renders the resolved block, so the card agrees with what the gateway will dial', async () => {
		stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		const field = await loaded();
		await fireEvent.input(field, {
			target: { value: 'http://127.0.0.1:8095/v1/audio/speech' }
		});
		await save();

		await waitFor(() => expect(screen.getByRole('status')).toBeTruthy());

		// The hint changed because the source did, which is the resolved block reaching the card rather than
		// the card keeping what it typed.
		expect(
			screen.getByText('Set here. Clear it to go back to the value the registry declares.')
		).toBeTruthy();
		expect(squashed(screen.getByRole('status'))).toBe(
			'Saved. This is the address the gateway will dial for this kind.'
		);
	});

	forEachCase(
		[
			{ name: 'names the saved state in the status role', status: 200, alert: null },
			{
				name: 'names a refusal the panel could not predict, in the error state',
				status: 400,
				alert: 'The gateway refused this value.'
			}
		],
		async ({ status, alert }) => {
			stubMediaProviders({ rows: [mediaRow()], writeStatus: status });
			renderKind('tts');

			await loaded();
			await save();

			if (alert === null) {
				await waitFor(() => expect(screen.getByRole('status')).toBeTruthy());
				expect(screen.queryByRole('alert')).toBeNull();
				return;
			}

			await waitFor(() => expect(screen.getByText(alert)).toBeTruthy());
		}
	);

	it('clears a previous refusal once a save succeeds, so a stale message cannot sit under a fresh one', async () => {
		const stub = stubMediaProviders({
			rows: [mediaRow({ base_url: '', base_url_source: 'registry' })],
			registry: { openai: { base_url: '', default_model: 'tts-1' } }
		});
		renderKind('tts');

		const field = await loaded();
		await save();
		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());

		await fireEvent.input(field, { target: { value: 'http://127.0.0.1:8095/v1/audio/speech' } });
		await save();

		// Waited on the status rather than on the alert's absence: the refusal clears the moment the save
		// starts, so an empty alert only proves the attempt began.
		await waitFor(() => expect(screen.getByRole('status')).toBeTruthy());
		expect(stub.patches.length).toBe(1);
		expect(screen.queryByRole('alert')).toBeNull();
	});
});
