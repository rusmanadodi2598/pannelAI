// Every control the Media Provider screen renders carries a pressed colour (DESIGN.md §3.5).
//
// Two of these were bare underlines with no state rule at all: the provider name that routes to the
// registry entry, and the six kind links a wrong address hands back. Both are pressed as often as the save
// beside them, and on the device with no hover they answered nothing whatsoever.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import MediaProvidersPage from '../../src/routes/media-providers/[kind]/+page.svelte';
import { mediaRow, stubMediaProviders } from '../support/media-stub';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button, a[data-slot="button"]'));
}

function renderKind(kind: string): void {
	render(MediaProvidersPage, { props: { params: { kind }, data: {} } });
}

describe('Media Provider control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control on the screen a pressed colour', async () => {
		stubMediaProviders({
			rows: [
				mediaRow(),
				mediaRow({ provider_id: 'deepgram', provider_name: 'Deepgram', models: [] })
			]
		});
		renderKind('tts');
		await screen.findByText('Deepgram');

		// The refresh, and each card's name link and its commit.
		const seen = controls();
		expect(seen).toHaveLength(5);
		expectEveryControlHasStates(seen, 'the Media Provider screen');
	});

	it('keeps the provider name a link, and gives it the states anyway', async () => {
		stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		const link = (await screen.findByRole('link', { name: 'OpenAI' })) as HTMLAnchorElement;
		expect(link.getAttribute('href')).toBe('/providers/openai');
		expectControlStates(link);
	});

	it('leaves the commit on the neutral ramp, because the accent belongs to one action', async () => {
		// Each card carries its own save, so a filled accent here would put the accent on as many controls as
		// the registry has providers for this kind (DESIGN.md §3.4).
		stubMediaProviders({ rows: [mediaRow()] });
		renderKind('tts');

		const commit = (await screen.findByRole('button', { name: 'Save' })) as HTMLButtonElement;
		expectControlStates(commit);
		expect(commit.className).toContain('border-[var(--color-border)]');
		expect(commit.className).not.toContain('bg-[var(--color-accent)]');
	});

	it('marks the commit busy while its write is in flight', async () => {
		// The neutral ramp fades a disabled control rather than holding a colour, so the only thing that says
		// "this one is working, not dead" is the attribute.
		stubMediaProviders({ rows: [mediaRow()], writePending: true });
		renderKind('tts');

		const commit = await screen.findByRole('button', { name: 'Save' });
		await fireEvent.click(commit);

		await waitFor(() => expect(commit.getAttribute('aria-busy')).toBe('true'));
		expect(commit.hasAttribute('disabled')).toBe(true);
	});

	it('gives the six kind links their addresses and their states', async () => {
		stubMediaProviders({ rows: [] });
		renderKind('chat');

		await screen.findByRole('alert');

		// The way out of a bad address is the list of real ones, and every entry in it is a press.
		const kinds: [string, string][] = [
			['Embedding', 'embedding'],
			['Image', 'image'],
			['Video', 'video'],
			['TTS', 'tts'],
			['STT', 'stt'],
			['Web Search', 'web']
		];

		for (const [label, kind] of kinds) {
			const link = screen.getByRole('link', { name: label }) as HTMLAnchorElement;
			expect(link.getAttribute('href'), `${label} link`).toBe(`/media-providers/${kind}`);
			expect(link.className).toContain('min-h-11');
			expectControlStates(link);
		}
	});

	it('gives a failed read its retry and its way back to the registry', async () => {
		stubMediaProviders({ rows: [mediaRow()], readStatus: 500 });
		renderKind('tts');

		expectControlStates(await screen.findByRole('button', { name: 'Try again' }));

		const back = screen.getByRole('link', { name: 'Back to the registry' }) as HTMLAnchorElement;
		expect(back.getAttribute('href')).toBe('/providers');
		expectControlStates(back);
	});

	it('gives the empty state its way to the registry', async () => {
		stubMediaProviders({ rows: [] });
		renderKind('image');

		const open = (await screen.findByRole('link', {
			name: 'Open Providers'
		})) as HTMLAnchorElement;
		expect(open.getAttribute('href')).toBe('/providers');
		expectControlStates(open);
	});
});
