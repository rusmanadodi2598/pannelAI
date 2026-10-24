// What a media provider save sends (docs/SPEC-UI/001-SPEC-UI.md §6.8, SPEC-API §7.10).
//
// The card edits two fields and one save carries both, so the assertions here are about the body that
// reached the wire: the kind in the API's spelling, the model the selector holds rather than the one the card
// was rendered with, and an empty field sent as an empty string because that is how an override is undone.
// What the card shows afterwards is in `media-providers-save.test.ts`.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import MediaProvidersPage from '../../src/routes/media-providers/[kind]/+page.svelte';
import { mediaRow, stubMediaProviders } from '../support/media-stub';
import { value } from '../support/dom';

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
