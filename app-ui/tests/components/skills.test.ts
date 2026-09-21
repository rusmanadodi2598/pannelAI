// Skills render tests (docs/SPEC-UI/001-SPEC-UI.md §6.10, R-27).
//
// The screen's contract is the catalog plus one honesty rule: §6.10 forbids a copy control that copies a
// broken link. So what is worth asserting is that the catalog decides what renders, and that an address
// which did not answer leaves no control and no link behind. The last tests are the check itself: the
// count is measured, and pressing it again really re-reads.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SkillsPage from '../../src/routes/skills/+page.svelte';
import { DEFAULT_ROWS, skillCatalog } from '../support/skill-catalog';

type SourceAnswer = number | 'network';

let catalogReads = 0;
let sourceReads: { url: string; method: string }[] = [];

function stubFetch(
	catalogBody: unknown,
	sourceAnswer: (url: string) => SourceAnswer = () => 200,
	catalogStatus = 200
): void {
	vi.stubGlobal('fetch', async (input: unknown, init?: { method?: string }) => {
		const url = String(input);

		if (url.startsWith('/api/v1/skills')) {
			catalogReads += 1;
			return new Response(JSON.stringify(catalogBody), {
				status: catalogStatus,
				headers: { 'content-type': 'application/json' }
			});
		}

		sourceReads.push({ url, method: init?.method ?? 'GET' });
		const answer = sourceAnswer(url);

		if (answer === 'network') throw new Error('Failed to fetch');

		return new Response(null, { status: answer });
	});
}

/** The entry block, which is a section rather than a list row. */
function entryBlock(): HTMLElement {
	const heading = screen.getByRole('heading', { name: 'Paste this into an AI client' });
	return heading.closest('section') as HTMLElement;
}

/** The list row for one capability, found by the id its addresses carry. */
function rowFor(id: string): HTMLElement {
	const row = screen
		.getAllByRole('listitem')
		.find((item) => item.textContent?.includes(`/skills/${id}/`));

	if (!row) throw new Error(`no list row for ${id}`);

	return row as HTMLElement;
}

beforeEach(() => {
	catalogReads = 0;
	sourceReads = [];
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	delete (navigator as { clipboard?: unknown }).clipboard;
});

describe('SkillsPage states', () => {
	it('says what it is loading while the catalog read is in flight', () => {
		vi.stubGlobal('fetch', () => new Promise(() => {}));
		render(SkillsPage);

		expect(screen.getByText('Loading the skill catalog')).toBeTruthy();
	});

	it('reports a failed catalog read and reads again when asked', async () => {
		stubFetch({ error: { code: 'UNAUTHORIZED', message: 'Session expired.' } }, () => 200, 401);
		render(SkillsPage);

		expect(await screen.findByText('The skill catalog could not be read')).toBeTruthy();
		expect(screen.getByText('Session expired.')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		expect(catalogReads).toBe(2);
	});

	it('states an empty catalog instead of rendering an empty page', async () => {
		stubFetch(skillCatalog([]));
		render(SkillsPage);

		expect(await screen.findByText('The gateway serves no skill')).toBeTruthy();
		expect(screen.getByText(/nothing to hand out here/)).toBeTruthy();
	});

	it('states the catalog it read', async () => {
		stubFetch(skillCatalog());
		render(SkillsPage);

		expect(await screen.findByText('Read from GET /skills.')).toBeTruthy();
		expect(screen.getByText('3 rows')).toBeTruthy();
	});
});

describe('SkillsPage entry skill', () => {
	it('hands out the install line, composed from the entry row own address', async () => {
		const copied: string[] = [];
		Object.defineProperty(navigator, 'clipboard', {
			value: {
				writeText: async (value: string) => {
					copied.push(value);
				}
			},
			configurable: true
		});
		stubFetch(skillCatalog());
		render(SkillsPage);

		const line = `Read this skill and use it: ${DEFAULT_ROWS[0].raw_url}`;
		await screen.findByText(line);

		const block = within(entryBlock());
		expect(block.getByText('Source available')).toBeTruthy();

		await fireEvent.click(block.getByRole('button', { name: 'Copy install line' }));

		expect(copied[0]).toBe(line);
	});

	it('withholds the install line when the entry skill is not published, and names the cause', async () => {
		stubFetch(skillCatalog(), (url) => (url.includes('/pannelai/') ? 404 : 200));
		render(SkillsPage);

		expect(await screen.findByText(/has no file at that path \(HTTP 404\)/)).toBeTruthy();

		const block = within(entryBlock());
		expect(block.queryByRole('button', { name: 'Copy install line' })).toBeNull();
		expect(block.getByText('Source unavailable')).toBeTruthy();
		// The address stays on screen: it names the path that has to be published.
		expect(block.getByText(DEFAULT_ROWS[0].raw_url)).toBeTruthy();
	});

	it('states that the catalog carries no entry skill rather than inventing a line', async () => {
		stubFetch(skillCatalog(DEFAULT_ROWS.slice(1)));
		render(SkillsPage);

		expect(await screen.findByText(/carries no entry skill/)).toBeTruthy();
		expect(screen.queryByRole('heading', { name: 'Paste this into an AI client' })).toBeNull();
	});
});

describe('SkillsPage capability rows', () => {
	it('renders one row per capability, with its route and its two addresses', async () => {
		stubFetch(skillCatalog());
		render(SkillsPage);

		// The rows render as soon as the catalog lands; the addresses wait for the check, so the count is
		// what the assertions below are anchored to.
		await screen.findByText('3 of 3 sources are published at the ref the catalog names.');

		const chat = within(rowFor('pannelai-chat'));
		expect(chat.getByText('Chat')).toBeTruthy();
		expect(chat.getByText('/chat/completions')).toBeTruthy();
		expect(chat.getByText(DEFAULT_ROWS[1].raw_url)).toBeTruthy();

		const link = chat.getByRole('link', { name: 'Read it on GitHub' }) as HTMLAnchorElement;
		expect(link.getAttribute('href')).toBe(DEFAULT_ROWS[1].blob_url);
	});

	it('leaves no copy control and no link on a row whose document did not answer', async () => {
		stubFetch(skillCatalog(), (url) => (url.includes('/pannelai-image/') ? 404 : 200));
		render(SkillsPage);

		await screen.findByText('2 of 3 sources are published at the ref the catalog names.');

		const image = within(rowFor('pannelai-image'));
		expect(image.getByText('Source unavailable')).toBeTruthy();
		expect(image.getByText(/has no file at that path \(HTTP 404\)/)).toBeTruthy();
		expect(image.queryByRole('button', { name: 'Copy install line' })).toBeNull();
		expect(image.queryByRole('link', { name: 'Read it on GitHub' })).toBeNull();

		// The row that did answer keeps its control, so the rule is per row rather than per page.
		expect(
			within(rowFor('pannelai-chat')).getByRole('button', { name: 'Copy install line' })
		).toBeTruthy();
	});

	it('asks the source host for the file without downloading it', async () => {
		stubFetch(skillCatalog());
		render(SkillsPage);

		await screen.findByText('Capability skills');

		expect(sourceReads.length).toBe(3);
		expect(sourceReads.every((read) => read.method === 'HEAD')).toBe(true);
	});
});

describe('SkillsPage source check', () => {
	it('reports the measured count rather than claiming every source is fine', async () => {
		stubFetch(skillCatalog(), (url) => (url.includes('/pannelai-image/') ? 500 : 200));
		render(SkillsPage);

		expect(
			await screen.findByText('2 of 3 sources are published at the ref the catalog names.')
		).toBeTruthy();
		expect(screen.getByText(/answered HTTP 500/)).toBeTruthy();
	});

	it('re-reads every source when Check again is pressed', async () => {
		let imageStatus: SourceAnswer = 404;
		stubFetch(skillCatalog(), (url) => (url.includes('/pannelai-image/') ? imageStatus : 200));
		render(SkillsPage);

		expect(
			await screen.findByText('2 of 3 sources are published at the ref the catalog names.')
		).toBeTruthy();

		imageStatus = 200;
		await fireEvent.click(screen.getByRole('button', { name: 'Check again' }));

		expect(
			await screen.findByText('3 of 3 sources are published at the ref the catalog names.')
		).toBeTruthy();
		expect(sourceReads.length).toBe(6);
		expect(
			within(rowFor('pannelai-image')).getByRole('button', { name: 'Copy install line' })
		).toBeTruthy();
	});

	it('reports a request that never completed rather than calling the file broken', async () => {
		stubFetch(skillCatalog(), () => 'network');
		render(SkillsPage);

		expect((await screen.findAllByText(/did not complete: Failed to fetch/)).length).toBe(3);
		expect(
			screen.getByText('0 of 3 sources are published at the ref the catalog names.')
		).toBeTruthy();
	});
});
