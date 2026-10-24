// The API Docs screen's controls carry the states (DESIGN.md §3.5).
//
// Two shapes were written at the call site: the retry a failed document read leaves behind, and the jump list
// that is the screen's table of contents. The jump list is dense, so it is the one place where the 44px row
// costs real height, and it still takes it: a reference opened on a phone is read by touching one of those
// entries, and a bare underline answers nothing under a finger.

import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ApiDocsPage from '../../src/routes/api-docs/+page.svelte';
import { openapiDocument } from '../support/openapi-document';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

function stubDocument(body: unknown, status = 200): void {
	vi.stubGlobal(
		'fetch',
		async () =>
			new Response(JSON.stringify(body), {
				status,
				headers: { 'content-type': 'application/json' }
			})
	);
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('API Docs control states', () => {
	it('gives the failed read its retry, on the ramp and still an underlined action', async () => {
		stubDocument({ error: { code: 'UNAUTHORIZED', message: 'Session expired.' } }, 401);
		render(ApiDocsPage);

		const retry = await screen.findByRole('button', { name: 'Try again' });
		expectControlStates(retry);
		expect(retry.className).toContain('underline');
		expect(retry.className).toContain('min-h-11');
	});

	it('gives every jump-list entry its address, its states, and a 44px row', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		await waitFor(() => expect(screen.getAllByRole('link').length).toBeGreaterThan(0));

		const jumps = screen.getAllByRole('link');
		for (const jump of jumps) {
			expectControlStates(jump);
			expect(jump.getAttribute('href'), `${jump.textContent} has no anchor`).toMatch(/^#/);
			expect(jump.className, `${jump.textContent} lost its 44px row`).toContain('min-h-11');
		}
	});

	it('gives every button on the screen a hover and a pressed step', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		await screen.findByRole('heading', { level: 1 });
		await waitFor(() => expect(document.querySelectorAll('button').length).toBeGreaterThan(0));

		expectEveryControlHasStates(
			Array.from(document.querySelectorAll('button')),
			'the API Docs screen'
		);
	});

	it('names each of its Copy controls by what it copies, and gives each a 44px target', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		await screen.findByRole('heading', { name: /Gateway Keys/ });

		// One base URL and one example per group: a page of identical "Copy" controls tells a screen
		// reader nothing about which value it is about to overwrite the clipboard with.
		const copies = screen.getAllByRole('button', { name: /^Copy / });
		expect(copies.length).toBe(5);

		const names = copies.map((copy) => copy.getAttribute('aria-label'));
		expect(new Set(names).size).toBe(names.length);

		for (const copy of copies) {
			expect(copy.textContent?.trim(), `${copy.getAttribute('aria-label')} shows text`).toBe('');
			expect(copy.className, `${copy.getAttribute('aria-label')} is under 44px`).toContain(
				'size-11'
			);
		}
	});
});
