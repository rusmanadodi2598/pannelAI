// Skills control names and per-row source answers (docs/SPEC-UI/001-SPEC-UI.md §6.10).
//
// Three icon-only copy controls on one page need three distinct accessible names, and the same is true of
// the three source links. The source check is per row too: the entry install line must not wait for the
// slowest capability document before it can be copied.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SkillsPage from '../../src/routes/skills/+page.svelte';
import { DEFAULT_ROWS, skillCatalog } from '../support/skill-catalog';
import { stubSkills } from '../support/skills-stub';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('Skills control names', () => {
	it('names each copy control for the skill it hands out', async () => {
		stubSkills(skillCatalog());
		render(SkillsPage);
		await screen.findByText('3 of 3 sources are published.');

		const copies = screen.getAllByRole('button', { name: /Copy install line/ });
		const names = copies.map(
			(button) => button.getAttribute('aria-label') ?? button.textContent?.trim() ?? ''
		);

		expect(copies).toHaveLength(3);
		expect(new Set(names).size).toBe(3);
		for (const name of names) expect(name).toContain('Copy install line');
	});

	it('keeps every copy control icon-only with a 44px target and its name on hover', async () => {
		stubSkills(skillCatalog());
		render(SkillsPage);
		await screen.findByText('3 of 3 sources are published.');

		for (const button of screen.getAllByRole('button', { name: /Copy install line/ })) {
			expect(button.textContent?.trim()).toBe('');
			expect(button.getAttribute('title')).toBe(button.getAttribute('aria-label'));
			expect(button.className).toContain('size-11');
		}
	});

	it('names each source link for the skill it opens', async () => {
		stubSkills(skillCatalog());
		render(SkillsPage);
		await screen.findByText('3 of 3 sources are published.');

		const links = screen.getAllByRole('link', { name: /Read it on GitHub/ });
		const names = links.map(
			(link) => link.getAttribute('aria-label') ?? link.textContent?.trim() ?? ''
		);

		expect(new Set(names).size).toBe(3);
		for (const name of names) expect(name).toContain('Read it on GitHub');
	});
});

describe('Skills source summary', () => {
	it('counts a one-row catalog in the singular', async () => {
		stubSkills(skillCatalog(DEFAULT_ROWS.slice(0, 1)));
		render(SkillsPage);

		expect(await screen.findByText('1 of 1 source is published.')).toBeTruthy();
	});
});

describe('Skills source answers', () => {
	it('shows the entry line while slower capability sources are still being checked', async () => {
		let release!: () => void;
		const held = new Promise<void>((resolve) => {
			release = resolve;
		});
		const catalog = skillCatalog();

		vi.stubGlobal('fetch', async (input: unknown) => {
			const url = String(input);
			if (url.startsWith('/api/v1/skills')) {
				return new Response(JSON.stringify(catalog), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				});
			}

			// The entry row answers first; the two capabilities stay open until the assertion has seen it.
			if (!/\/pannelai\/SKILL\.md$/.test(url)) await held;
			return new Response(null, { status: 200 });
		});

		render(SkillsPage);

		expect(
			await screen.findByRole('button', { name: 'Copy install line for pannelAI (Entry)' })
		).toBeTruthy();
		expect(screen.getByText('Checking 3 sources')).toBeTruthy();

		release();
		expect(await screen.findByText('3 of 3 sources are published.')).toBeTruthy();
	});
});
