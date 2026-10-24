// Media Provider page tests (docs/SPEC-UI/001-SPEC-UI.md §6.8).
//
// The screen is one page at six addresses, so these tests drive the page rather than a component in
// isolation: the kind comes from the route, and a test that rendered a card directly could not tell whether
// the page passed the right kind to the API. What the card shows is `media-provider-card.test.ts`, and the
// save is `media-providers-save.test.ts`.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import MediaProvidersPage from '../../src/routes/media-providers/[kind]/+page.svelte';
import { mediaRow, stubMediaProviders } from '../support/media-stub';
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
		expect(screen.getByText('Pick one of the six media kinds.')).toBeTruthy();
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
