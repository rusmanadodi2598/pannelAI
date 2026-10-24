// What one media provider card shows (docs/SPEC-UI/001-SPEC-UI.md §6.8).
//
// The card is driven through the page, not rendered alone, because the kind comes from the route: a test
// that mounted the card directly could not tell whether the page passed the right kind to the API. What the
// page renders and what it requests are asserted together for the same reason.
//
// The save itself is `media-providers-save.test.ts`.

import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
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

	it('wraps a long provider name and id inside the card', async () => {
		const longName =
			'TH HARBOR 1 with a very long custom provider display name for layout pressure';
		const longId = 'openai-compatible-0386BKG9Q4DYZPYC01VJH4C51G';
		stubMediaProviders({
			rows: [mediaRow({ provider_name: longName, provider_id: longId, endpoint_count: 1 })]
		});
		renderKind('tts');

		const link = await screen.findByRole('link', { name: longName });
		expect(link.className).toContain('whitespace-normal');
		expect(link.className).toContain('w-full');
		expect(screen.getByText(longName).className).toContain('break-words');
		expect(screen.getByText(longId).className).toContain('break-all');
		expect(screen.getByText('1 endpoint for this kind').className).toContain('max-sm:basis-full');
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
			'Set here. Clear it to use the registry value.'
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
			expect(screen.getByText('No models here, so the registry default applies.')).toBeTruthy()
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
				screen.getByText('Currently tts-0, no longer declared. The registry default clears it.')
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
