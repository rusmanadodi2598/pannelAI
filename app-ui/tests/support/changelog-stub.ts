// The changelog screen's two reads, faked for every test that renders it.
//
// The screen is one comparison over two routes, so a test that stubs only the releases is testing half the
// screen. Both answers and the recorder live here so the state walk and the render tests ask the same fake.

import { vi } from 'vitest';

export const VERSION = {
	version: 'v0.4.0',
	commit: 'abc1234',
	build_date: '2026-09-20T00:00:00Z',
	go_version: '1.26',
	registry_revision: '9router@db4499d'
};

export const RELEASES = [
	{ version: 'v0.4.0', date: '2026-09-20', title: 'Skills catalog', notes: 'Served.' },
	{ version: 'v0.3.0', date: '2026-09-19', title: 'Responses API', notes: 'Responses.' },
	{ version: 'v0.2.0', date: '2026-09-19', title: 'OAuth', notes: 'OAuth.' },
	{ version: 'v0.1.0', date: '2026-09-18', title: 'Registry', notes: 'Registry.' }
];

export type ChangelogStub = { reads: string[] };

/** Answers both routes the screen reads, and records every request so a re-read can be counted. */
export function stubChangelog(
	options: {
		releases?: unknown;
		releasesStatus?: number;
		version?: unknown;
		versionStatus?: number;
	} = {}
): ChangelogStub {
	const reads: string[] = [];
	const releases = options.releases ?? { data: RELEASES };
	const version = options.version ?? VERSION;

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		reads.push(url);

		const changelog = url.startsWith('/api/v1/changelog');
		return new Response(JSON.stringify(changelog ? releases : version), {
			status: changelog ? (options.releasesStatus ?? 200) : (options.versionStatus ?? 200),
			headers: { 'content-type': 'application/json' }
		});
	});

	return { reads };
}
