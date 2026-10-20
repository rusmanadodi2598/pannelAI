// The Skills screen's fetch harness, shared by the tests that render it.
//
// The screen reads two things: the catalog from the gateway, and one HEAD per source address. The stub
// keeps them apart because the rule under test is usually which of the two answered, and `sourceAnswers`
// records the probe calls so a test can say the check asked without downloading.

import { screen } from '@testing-library/svelte';
import { vi } from 'vitest';

export type SourceAnswer = number | 'network';

export type SkillsStub = {
	catalogReads: number;
	sources: { url: string; method: string }[];
};

export function stubSkills(
	catalogBody: unknown,
	sourceAnswer: (url: string) => SourceAnswer = () => 200,
	catalogStatus = 200
): SkillsStub {
	const stub: SkillsStub = { catalogReads: 0, sources: [] };

	vi.stubGlobal('fetch', async (input: unknown, init?: { method?: string }) => {
		const url = String(input);

		if (url.startsWith('/api/v1/skills')) {
			stub.catalogReads += 1;
			return new Response(JSON.stringify(catalogBody), {
				status: catalogStatus,
				headers: { 'content-type': 'application/json' }
			});
		}

		stub.sources.push({ url, method: init?.method ?? 'GET' });
		const answer = sourceAnswer(url);
		if (answer === 'network') throw new Error('Failed to fetch');

		return new Response(null, { status: answer });
	});

	return stub;
}

/** The entry block, which is a section rather than a list row. */
export function entryBlock(): HTMLElement {
	const heading = screen.getByRole('heading', { name: 'Paste this into an AI client' });
	return heading.closest('section') as HTMLElement;
}

/** The list row for one capability, found by the id its addresses carry. */
export function rowFor(id: string): HTMLElement {
	const row = screen
		.getAllByRole('listitem')
		.find((item) => item.textContent?.includes(`/skills/${id}/`));

	if (!row) throw new Error(`no list row for ${id}`);

	return row as HTMLElement;
}
