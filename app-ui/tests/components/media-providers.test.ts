// Media Provider screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.8).
//
// The screen is one page at six addresses, so the tests drive the page itself rather than a component in
// isolation: the kind comes from the route, and a test that rendered a card directly could not tell
// whether the page passed the right kind to the API. What the page renders and what it requests are
// asserted together, because a request for the wrong kind would look identical on screen.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import MediaProvidersPage from '../../src/routes/media-providers/[kind]/+page.svelte';
import { mediaRow, stubMediaProviders } from '../support/media-stub';
import { squashed, value } from '../support/dom';
import { forEachCase } from '../support/tables';

// A faithful stand-in for `resolve`: it fills the placeholders, so an assertion on an href is about the
// address the panel built rather than about the route pattern it started from.
vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

function renderKind(kind: string): void {
	render(MediaProvidersPage, { props: { params: { kind }, data: {} } });
}

/**
 * The hint a field points at with `aria-describedby`, read through the wiring rather than by searching
 * for the sentence. The assertion then covers the accessibility link as well as the copy, so a hint that
 * stopped being described would fail here instead of passing on a loose text match.
 */
function hintFor(field: HTMLElement): string {
	const id = field.getAttribute('aria-describedby');
	expect(id, 'the field describes no hint').toBeTruthy();

	const hint = document.getElementById(String(id));
	expect(hint, `no element carries the id ${id}`).toBeTruthy();

	return squashed(hint as HTMLElement);
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the page reads one kind', () => {
	forEachCase(
		[
			{ name: 'asks the API for tts when the address says tts', slug: 'tts', api: 'tts' },
			{
				name: 'asks the API for embedding when the address says embedding',
				slug: 'embedding',
				api: 'embedding'
			},
			// The one place the two vocabularies differ, and the reason the mapping exists at all.
			{ name: 'asks the API for search when the address says web', slug: 'web', api: 'search' },
			{ name: 'asks the API for video when the address says video', slug: 'video', api: 'video' }
		],
		async ({ slug, api }) => {
			const stub = stubMediaProviders({ rows: [] });
			renderKind(slug);

			await waitFor(() => expect(stub.reads.length).toBe(1));
			expect(stub.reads[0]).toBe(api);
		}
	);

	it('asks once for the kind the address names, and never for every kind', async () => {
		// The API accepts an absent kind and lists everything. The page is per kind, so it must filter:
		// rendering the whole registry under one kind's heading is the failure this rules out.
		const stub = stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		await waitFor(() => expect(stub.reads.length).toBe(1));
		expect(stub.reads[0]).not.toBe('');
	});

	it('does not ask at all when the address names no kind the panel has', async () => {
		// A bad URL is the panel's own problem. Asking anyway would report the API's 400 as if the registry
		// were at fault, which sends the operator looking in the wrong place.
		const stub = stubMediaProviders({ rows: [mediaRow()] });
		renderKind('chat');

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(stub.reads).toEqual([]);
		// §8.6.2's control is absent here for the same reason: there is no read to repeat, and a button that
		// silently did nothing would read as broken.
		expect(screen.queryByRole('button', { name: 'Refresh now' })).toBeNull();
	});

	it('re-reads the kind on screen when the operator asks for it', async () => {
		const stub = stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');
		await waitFor(() => expect(stub.reads.length).toBe(1));

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the same kind is asked for again, so an override saved elsewhere shows up without
		// re-entering the address.
		await waitFor(() => expect(stub.reads.length).toBe(2));
		expect(stub.reads[1]).toBe('tts');
	});
});

describe('the page states', () => {
	it('names the kind in the heading, not the slug', async () => {
		stubMediaProviders({ rows: [mediaRow()] });
		renderKind('web');

		await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toBeTruthy());
		expect(screen.getByRole('heading', { level: 1 }).textContent?.trim()).toBe('Web Search');
	});

	it('renders a card per provider the registry offers for the kind', async () => {
		stubMediaProviders({
			rows: [
				mediaRow(),
				mediaRow({ provider_id: 'deepgram', provider_name: 'Deepgram', kind: 'tts' })
			]
		});
		renderKind('tts');

		await waitFor(() => expect(screen.getByText('OpenAI')).toBeTruthy());
		expect(screen.getByText('Deepgram')).toBeTruthy();
		expect(screen.getByText('deepgram')).toBeTruthy();
	});

	it('sends the provider name to its registry entry, where its endpoints are', async () => {
		// The card states a count scoped to one kind, and the endpoint screen filters by provider alone, so
		// the count is not the thing that links. The name is, and it goes where the endpoints are listed.
		stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		await waitFor(() => expect(screen.getByRole('link', { name: 'OpenAI' })).toBeTruthy());
		expect(screen.getByRole('link', { name: 'OpenAI' }).getAttribute('href')).toBe(
			'/providers/openai'
		);
	});

	it('says the registry offers none, with the sentence §6.8 gives and a way to the registry', async () => {
		stubMediaProviders({ rows: [] });
		renderKind('image');

		await waitFor(() =>
			expect(screen.getByText('No provider configured for this kind.')).toBeTruthy()
		);

		const link = screen.getByRole('link', { name: 'Open Providers' });
		expect(link.getAttribute('href')).toBe('/providers');
	});

	it('names the six kinds when the address names one that does not exist', async () => {
		stubMediaProviders({ rows: [] });
		renderKind('chat');

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());

		// The way out is the list of real addresses, so the operator picks one instead of guessing. The
		// labels are the sidebar's, because a link here and a row there name the same screen.
		const kinds: [string, string][] = [
			['Embedding', 'embedding'],
			['Image', 'image'],
			['Video', 'video'],
			['TTS', 'tts'],
			['STT', 'stt'],
			['Web Search', 'web']
		];

		for (const [label, slug] of kinds) {
			const link = screen.getByRole('link', { name: label });
			expect(link.getAttribute('href'), `${label} link`).toBe(`/media-providers/${slug}`);
		}
	});

	it('offers a retry when the registry cannot be read', async () => {
		const stub = stubMediaProviders({ rows: [], readStatus: 500 });
		renderKind('tts');

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(screen.getByText('The registry is unreachable.')).toBeTruthy();

		stub.readStatus = 200;
		screen.getByRole('button', { name: 'Try again' }).click();

		await waitFor(() => expect(stub.reads.length).toBe(2));
	});
});

describe('what a card shows', () => {
	it('leaves the base URL field empty when the registry is serving the value, and names that value', async () => {
		// The field is the override, not the resolved value. Pre-filling it would write an override on an
		// untouched save, which changes the source without changing behaviour.
		stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		await waitFor(() => expect(screen.getByLabelText('Base URL')).toBeTruthy());

		expect(value(screen.getByLabelText('Base URL'))).toBe('');
		expect(hintFor(screen.getByLabelText('Base URL'))).toBe(
			'The registry declares https://api.openai.com/v1/audio/speech. Leave this empty to use it.'
		);
	});

	it('fills the base URL field when the override is what the gateway is dialing', async () => {
		stubMediaProviders({
			rows: [
				mediaRow({
					base_url: 'http://127.0.0.1:8095/v1/audio/speech',
					base_url_source: 'override'
				})
			],
			registry: {
				openai: { base_url: 'https://api.openai.com/v1/audio/speech', default_model: 'tts-1' }
			}
		});
		renderKind('tts');

		await waitFor(() =>
			expect(value(screen.getByLabelText('Base URL'))).toBe('http://127.0.0.1:8095/v1/audio/speech')
		);
		expect(hintFor(screen.getByLabelText('Base URL'))).toBe(
			'Set here. Clear it to go back to the value the registry declares.'
		);
	});

	it('offers the declared models plus the registry default', async () => {
		stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		await waitFor(() => expect(screen.getByLabelText('Default model')).toBeTruthy());

		const options = Array.from(
			(screen.getByLabelText('Default model') as HTMLSelectElement).options
		).map((option) => option.textContent);
		expect(options).toEqual(['Use the registry default', 'TTS 1 (tts-1)', 'tts-1-hd']);
	});

	it('renders no selector when the kind declares no models, and says why', async () => {
		// A select with no options is a dead control (R-26). The fact replaces it.
		stubMediaProviders({ rows: [mediaRow({ kind: 'image', models: [] })] });
		renderKind('image');

		await waitFor(() =>
			expect(
				screen.getByText(
					'This service declares no models, so there is nothing to choose. The registry default applies.'
				)
			).toBeTruthy()
		);
		expect(screen.queryByLabelText('Default model')).toBeNull();
	});

	it('names a stored model the service no longer declares, and resets the selector to the default', async () => {
		stubMediaProviders({
			rows: [
				mediaRow({
					default_model: 'tts-0',
					default_model_source: 'override',
					models: [{ id: 'tts-1' }]
				})
			],
			registry: {
				openai: { base_url: 'https://api.openai.com/v1/audio/speech', default_model: '' }
			}
		});
		renderKind('tts');

		await waitFor(() =>
			expect(
				screen.getByText(
					'Currently set to tts-0, which this service no longer declares. Choosing the registry default clears it.'
				)
			).toBeTruthy()
		);
		expect(value(screen.getByLabelText('Default model'))).toBe('');
	});

	forEachCase(
		[
			{
				name: 'says how many endpoints route to the provider',
				count: 3,
				text: '3 endpoints for this kind'
			},
			{ name: 'uses the singular for one endpoint', count: 1, text: '1 endpoint for this kind' },
			{
				name: 'names the zero case, which is the one an operator has to act on',
				count: 0,
				text: '0 endpoints configured for this kind yet'
			}
		],
		async ({ count, text }) => {
			stubMediaProviders({ rows: [mediaRow({ endpoint_count: count })] });
			renderKind('tts');

			await waitFor(() => expect(screen.getByText(text)).toBeTruthy());
		}
	);

	it('never claims a fallback path the API does not implement', async () => {
		// §6.8 forbids the claim, and R-36 forbids a fabricated capability. The screen describes where a
		// value comes from, which is real, and says nothing about what happens when a provider fails.
		stubMediaProviders({
			rows: [
				mediaRow(),
				mediaRow({ provider_id: 'deepgram', provider_name: 'Deepgram', models: [] })
			]
		});
		renderKind('tts');

		await waitFor(() => expect(screen.getByText('OpenAI')).toBeTruthy());
		expect(screen.getByText('Deepgram')).toBeTruthy();

		const body = document.body.textContent ?? '';
		expect(body).not.toMatch(/fall ?back|fail ?over|retry with|backup provider/i);
	});
});
