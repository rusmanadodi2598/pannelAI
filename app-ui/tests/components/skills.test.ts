// Skills render tests (docs/SPEC-UI/001-SPEC-UI.md §6.10, R-27).
//
// The screen's contract is the catalog plus one honesty rule: §6.10 forbids a copy control that copies a
// broken link. So what is worth asserting is that the catalog decides what renders, and that an address
// which did not answer leaves no control and no link behind. The last tests are the check itself: the
// count is measured, and pressing it again really re-reads.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SkillsPage from '../../src/routes/skills/+page.svelte';
import { DEFAULT_ROWS, skillCatalog } from '../support/skill-catalog';
import { entryBlock, rowFor, stubSkills, type SourceAnswer } from '../support/skills-stub';

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
		const stub = stubSkills(
			{ error: { code: 'UNAUTHORIZED', message: 'Session expired.' } },
			() => 200,
			401
		);
		render(SkillsPage);

		expect(await screen.findByText('Could not read the skill catalog')).toBeTruthy();
		expect(screen.getByText('Session expired.')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		expect(stub.catalogReads).toBe(2);
	});

	it('states an empty catalog instead of rendering an empty page', async () => {
		stubSkills(skillCatalog([]));
		render(SkillsPage);

		expect(await screen.findByText('The gateway serves no skill')).toBeTruthy();
		expect(
			screen.getByText('The catalog returned no rows, so there is nothing to hand out.')
		).toBeTruthy();
	});

	it('states the catalog it read', async () => {
		stubSkills(skillCatalog());
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
		stubSkills(skillCatalog());
		render(SkillsPage);

		const line = `Read this skill and use it: ${DEFAULT_ROWS[0].raw_url}`;
		await screen.findByText(line);

		const block = within(entryBlock());
		expect(block.getByText('Source available')).toBeTruthy();

		await fireEvent.click(
			block.getByRole('button', { name: 'Copy install line for pannelAI (Entry)' })
		);

		expect(copied[0]).toBe(line);
	});

	it('withholds the install line when the entry skill is not published, and names the cause', async () => {
		stubSkills(skillCatalog(), (url) => (url.includes('/pannelai/') ? 404 : 200));
		render(SkillsPage);

		expect(
			await screen.findByText('No file at that path (HTTP 404). Publish it in the repository.')
		).toBeTruthy();

		const block = within(entryBlock());
		expect(block.queryByRole('button', { name: /Copy install line/ })).toBeNull();
		expect(block.getByText('Source unavailable')).toBeTruthy();
		// The address stays on screen: it names the path that has to be published.
		expect(block.getByText(DEFAULT_ROWS[0].raw_url)).toBeTruthy();
	});

	it('states that the catalog carries no entry skill rather than inventing a line', async () => {
		stubSkills(skillCatalog(DEFAULT_ROWS.slice(1)));
		render(SkillsPage);

		expect(
			await screen.findByText('No entry skill here. The rows below list its capabilities.')
		).toBeTruthy();
		expect(screen.queryByRole('heading', { name: 'Paste this into an AI client' })).toBeNull();
	});
});

describe('SkillsPage capability rows', () => {
	it('renders one row per capability, with its route and its two addresses', async () => {
		stubSkills(skillCatalog());
		render(SkillsPage);

		// The rows render as soon as the catalog lands; the addresses wait for the check, so the count is
		// what the assertions below are anchored to.
		await screen.findByText('3 of 3 sources are published.');

		const chat = within(rowFor('pannelai-chat'));
		expect(chat.getByText('Chat')).toBeTruthy();
		expect(chat.getByText('/chat/completions')).toBeTruthy();
		expect(chat.getByText(DEFAULT_ROWS[1].raw_url)).toBeTruthy();

		const link = chat.getByRole('link', {
			name: 'Read it on GitHub: Chat'
		}) as HTMLAnchorElement;
		expect(link.getAttribute('href')).toBe(DEFAULT_ROWS[1].blob_url);
	});

	it('leaves no copy control and no link on a row whose document did not answer', async () => {
		stubSkills(skillCatalog(), (url) => (url.includes('/pannelai-image/') ? 404 : 200));
		render(SkillsPage);

		await screen.findByText('2 of 3 sources are published.');

		const image = within(rowFor('pannelai-image'));
		expect(image.getByText('Source unavailable')).toBeTruthy();
		expect(
			image.getByText('No file at that path (HTTP 404). Publish it in the repository.')
		).toBeTruthy();
		expect(image.queryByRole('button', { name: /Copy install line/ })).toBeNull();
		expect(image.queryByRole('link', { name: /Read it on GitHub/ })).toBeNull();

		// The row that did answer keeps its control, so the rule is per row rather than per page.
		expect(
			within(rowFor('pannelai-chat')).getByRole('button', {
				name: 'Copy install line for Chat'
			})
		).toBeTruthy();
	});

	it('asks the source host for the file without downloading it', async () => {
		const stub = stubSkills(skillCatalog());
		render(SkillsPage);

		await screen.findByText('Capability skills');

		expect(stub.sources.length).toBe(3);
		expect(stub.sources.every((read) => read.method === 'HEAD')).toBe(true);
	});
});

describe('SkillsPage source check', () => {
	it('reports the measured count rather than claiming every source is fine', async () => {
		stubSkills(skillCatalog(), (url) => (url.includes('/pannelai-image/') ? 500 : 200));
		render(SkillsPage);

		expect(await screen.findByText('2 of 3 sources are published.')).toBeTruthy();
		expect(screen.getByText(/answered HTTP 500/)).toBeTruthy();
	});

	it('re-reads every source when Check again is pressed', async () => {
		let imageStatus: SourceAnswer = 404;
		const stub = stubSkills(skillCatalog(), (url) =>
			url.includes('/pannelai-image/') ? imageStatus : 200
		);
		render(SkillsPage);

		expect(await screen.findByText('2 of 3 sources are published.')).toBeTruthy();

		imageStatus = 200;
		await fireEvent.click(screen.getByRole('button', { name: 'Check again' }));

		expect(await screen.findByText('3 of 3 sources are published.')).toBeTruthy();
		expect(stub.sources.length).toBe(6);
		expect(
			within(rowFor('pannelai-image')).getByRole('button', {
				name: 'Copy install line for Image Generation'
			})
		).toBeTruthy();
	});

	it('reports a request that never completed rather than calling the file broken', async () => {
		stubSkills(skillCatalog(), () => 'network');
		render(SkillsPage);

		expect((await screen.findAllByText(/did not complete: Failed to fetch/)).length).toBe(3);
		expect(screen.getByText('0 of 3 sources are published.')).toBeTruthy();
	});
});
