// API Docs render tests (docs/SPEC-UI/001-SPEC-UI.md §6.12, R-27).
//
// The screen is a rendering of the document, so what is worth asserting is that the document decides:
// the catalog and its groups, the credential per row, the example composed from the operation, the error
// table with each plane's own wording, and the three states. The last test is the honesty rule §6.12
// states about keys: no real key can appear anywhere on the screen.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ApiDocsPage from '../../src/routes/api-docs/+page.svelte';
import { openapiDocument } from '../support/openapi-document';

function stubDocument(body: unknown, status = 200): string[] {
	const requested: string[] = [];

	vi.stubGlobal('fetch', async (input: unknown) => {
		requested.push(String(input));
		return new Response(JSON.stringify(body), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return requested;
}

function stubClipboard(writeText: (value: string) => Promise<void>): void {
	Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	delete (navigator as { clipboard?: unknown }).clipboard;
});

describe('ApiDocsPage states', () => {
	it('says what it is loading while the read is in flight', () => {
		vi.stubGlobal('fetch', () => new Promise(() => {}));
		render(ApiDocsPage);

		expect(screen.getByText('Loading the served contract')).toBeTruthy();
	});

	it('reports a failed read and reads again when asked', async () => {
		const requested = stubDocument(
			{ error: { code: 'UNAUTHORIZED', message: 'Session expired.' } },
			401
		);
		render(ApiDocsPage);

		expect(await screen.findByText('The contract could not be read')).toBeTruthy();
		expect(screen.getByText('Session expired.')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		expect(requested.length).toBe(2);
	});

	it('reports a document that declares no path instead of an empty page', async () => {
		stubDocument(openapiDocument({ paths: {} }));
		render(ApiDocsPage);

		expect(await screen.findByText('This document declares no paths')).toBeTruthy();
		expect(screen.getByText(/nothing to render here/)).toBeTruthy();
	});

	it('states the document it read', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		expect(await screen.findByText('pannelAI API')).toBeTruthy();
		expect(screen.getByText('Document version v0.4.0')).toBeTruthy();
		expect(screen.getByText('OpenAPI 3.1.0')).toBeTruthy();
		expect(screen.getByText('Read from GET /openapi.json.')).toBeTruthy();
	});
});

describe('ApiDocsPage catalog', () => {
	it('renders one section per group, in the document order, with the untagged tail last', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		await screen.findByRole('heading', { name: /Gateway Keys/ });

		const headings = screen
			.getAllByRole('heading', { level: 3 })
			// The error tables use level 3 as well; the catalog's own headings carry the group anchors.
			.filter((heading) => heading.id.startsWith('group-'))
			.map((heading) => heading.textContent?.trim().split(' ')[0]);

		expect(headings).toEqual(['System', 'Gateway', 'Data', 'Untagged']);
	});

	it('shows the operation, its summary, and the credential it declares', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		const table = within(await screen.findByRole('table', { name: /Operations in Gateway Keys/ }));

		expect(table.getByText('get')).toBeTruthy();
		// The group holds two operations on one path, which is why the count is asserted rather than a match.
		expect(table.getAllByText('/api/v1/gateway-keys').length).toBe(2);
		expect(table.getByText('/api/v1/gateway-keys/{id}')).toBeTruthy();
		expect(table.getByText('List keys')).toBeTruthy();
		expect(table.getAllByText('Session cookie').length).toBe(3);
	});

	it('marks a public operation as carrying no credential', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		const table = within(await screen.findByRole('table', { name: /Operations in System/ }));

		expect(table.getByText('No credential')).toBeTruthy();
	});

	it('composes the group example from the operation and copies exactly it', async () => {
		const copied: string[] = [];
		stubClipboard(async (value) => {
			copied.push(value);
		});
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		const heading = await screen.findByRole('heading', { name: /Gateway Keys/ });
		const section = within(heading.closest('section') as HTMLElement);

		expect(
			section.getByText(/curl -X GET 'http:\/\/localhost:8080\/api\/v1\/gateway-keys'/)
		).toBeTruthy();
		expect(section.getByText(/pannel_session=<session cookie>/)).toBeTruthy();

		await fireEvent.click(section.getByRole('button', { name: 'Copy' }));

		expect(copied[0]).toContain("curl -X GET 'http://localhost:8080/api/v1/gateway-keys'");
		await vi.waitFor(() => {
			expect(section.getByText('Copied.')).toBeTruthy();
		});
	});

	it('says the copy failed instead of claiming it worked', async () => {
		stubClipboard(async () => {
			throw new Error('not allowed');
		});
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		const heading = await screen.findByRole('heading', { name: /Gateway Keys/ });
		const section = within(heading.closest('section') as HTMLElement);

		await fireEvent.click(section.getByRole('button', { name: 'Copy' }));

		await vi.waitFor(() => {
			expect(section.getByText('Copy failed. Select the text and copy it.')).toBeTruthy();
		});
	});

	it('states that the document declares no base URL rather than borrowing one', async () => {
		stubDocument(openapiDocument({ servers: undefined }));
		render(ApiDocsPage);

		expect(await screen.findByText(/This document declares no base URL/)).toBeTruthy();
		expect(screen.getByText(/curl -X GET '<gateway base URL>\/api\/v1\/health'/)).toBeTruthy();
	});
});

describe('ApiDocsPage credentials and errors', () => {
	it('lists each scheme with its placement and how many operations use it', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		expect(await screen.findByText('cookie pannel_session')).toBeTruthy();
		expect(screen.getByText('bearer (gateway-key)')).toBeTruthy();
		expect(screen.getByText('Used by 4 operations.')).toBeTruthy();
		expect(screen.getByText('Used by 1 operation.')).toBeTruthy();
		expect(screen.getByText('One operation declares no credential.')).toBeTruthy();
	});

	it('renders one error table per plane with each plane own wording', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);

		const management = within(await screen.findByRole('table', { name: /Management plane/ }));
		expect(management.getByText('VALIDATION_ERROR')).toBeTruthy();
		expect(management.getByText('400')).toBeTruthy();
		expect(management.getByText('The request body or query failed validation.')).toBeTruthy();

		const dataPlane = within(screen.getByRole('table', { name: /Data plane/ }));
		expect(dataPlane.getByText('The gateway key was missing or invalid.')).toBeTruthy();
		expect(dataPlane.getByText('No description in the document.')).toBeTruthy();
	});

	it('says the document carries no error table when it has no contract block', async () => {
		stubDocument(openapiDocument({ 'x-contract': undefined }));
		render(ApiDocsPage);

		expect(await screen.findByText(/carries no error code table/)).toBeTruthy();
	});

	it('shows no key material anywhere on the screen', async () => {
		stubDocument(openapiDocument());
		render(ApiDocsPage);
		await screen.findByRole('heading', { name: /Data Plane/ });

		// §6.12: the examples carry an `sk-...` placeholder and never a real key.
		expect(document.body.textContent).toContain('sk-...');
		expect(document.body.textContent).not.toMatch(/sk-(?!\.\.\.)[A-Za-z0-9]/);
	});
});
