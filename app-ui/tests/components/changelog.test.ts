// Changelog render tests (docs/SPEC-UI/001-SPEC-UI.md §6.16).
//
// The screen is two reads and one comparison: the releases come from the served route, the running version
// decides what each release is marked, and a read that failed says which half failed instead of leaving a
// blank where a fact belongs. The last tests are the two ways the version half can go wrong, which are
// different sentences because they are different facts: a request that did not answer, and a value that is
// not a release number.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ChangelogPage from '../../src/routes/changelog/+page.svelte';

const VERSION = {
	version: 'v0.4.0',
	commit: 'abc1234',
	build_date: '2026-09-20T00:00:00Z',
	go_version: '1.26',
	registry_revision: '9router@db4499d'
};

const RELEASES = [
	{ version: 'v0.4.0', date: '2026-09-20', title: 'Skills catalog', notes: 'Served.' },
	{ version: 'v0.3.0', date: '2026-09-19', title: 'Responses API', notes: 'Responses.' },
	{ version: 'v0.2.0', date: '2026-09-19', title: 'OAuth', notes: 'OAuth.' },
	{ version: 'v0.1.0', date: '2026-09-18', title: 'Registry', notes: 'Registry.' }
];

let reads: string[] = [];

function stubFetch(
	options: {
		releases?: unknown;
		releasesStatus?: number;
		version?: unknown;
		versionStatus?: number;
	} = {}
): void {
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
}

/** The list row for one release, found by the version it carries. */
function rowFor(version: string): HTMLElement {
	const row = screen.getAllByRole('listitem').find((item) => item.textContent?.includes(version));

	if (!row) throw new Error(`no list row for ${version}`);

	return row as HTMLElement;
}

beforeEach(() => {
	reads = [];
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('ChangelogPage states', () => {
	it('says what it is loading while both reads are in flight', () => {
		vi.stubGlobal('fetch', () => new Promise(() => {}));
		render(ChangelogPage);

		expect(screen.getByText('Loading release notes')).toBeTruthy();
		// The version slot says it is reading, because "unknown" is a claim about a read that finished.
		expect(screen.getByText('Reading')).toBeTruthy();
	});

	it('reports a failed release read and reads both routes again when asked', async () => {
		stubFetch({
			releases: { error: { code: 'INTERNAL_ERROR', message: 'The gateway is down.' } },
			releasesStatus: 500
		});
		render(ChangelogPage);

		expect(await screen.findByText('Release notes could not be read')).toBeTruthy();
		expect(screen.getByText('The gateway is down.')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		expect(reads.filter((url) => url.startsWith('/api/v1/changelog')).length).toBe(2);
	});

	it('reads both routes again when the operator asks for a refresh', async () => {
		stubFetch();
		render(ChangelogPage);
		await screen.findByText('Skills catalog');

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the control repeats both reads, so a release published since the page was opened appears
		// without a reload, and the comparison is redone against the same build.
		await waitFor(() =>
			expect(reads.filter((url) => url.startsWith('/api/v1/changelog')).length).toBe(2)
		);
		expect(reads.filter((url) => url.startsWith('/api/v1/version')).length).toBe(2);
	});

	it('states an empty release list rather than a missing source', async () => {
		stubFetch({ releases: { data: [] } });
		render(ChangelogPage);

		expect(await screen.findByText('The gateway serves no release notes')).toBeTruthy();
		expect(screen.getByText(/answered with an empty list/)).toBeTruthy();
	});

	it('reads the release route and the version route', async () => {
		stubFetch();
		render(ChangelogPage);

		await screen.findByText('Read from GET /changelog.');
		expect(reads.some((url) => url.startsWith('/api/v1/version'))).toBe(true);
	});

	it('reports a contract drift instead of rendering the shape it used to demand', async () => {
		// The shape this panel used to demand: no title, no notes, and a category nobody serves.
		const oldShape = {
			version: 'v0.4.0',
			released_at: '2026-09-20',
			category: 'feature',
			items: ['x']
		};
		stubFetch({ releases: { data: [oldShape] } });
		render(ChangelogPage);

		expect(await screen.findByText('Release notes could not be read')).toBeTruthy();
		expect(screen.getByText(/Unexpected response from the gateway/)).toBeTruthy();
		expect(screen.queryByRole('listitem')).toBeNull();
	});
});

describe('ChangelogPage releases', () => {
	it('renders each served release with its date, its title, and its note', async () => {
		stubFetch();
		render(ChangelogPage);

		await screen.findByText('Skills catalog');
		expect(screen.getByText('4 releases')).toBeTruthy();
		expect(screen.getAllByRole('listitem').length).toBe(4);

		const newest = within(rowFor('v0.4.0'));
		expect(newest.getByText('2026-09-20')).toBeTruthy();
		expect(newest.getByText('Served.')).toBeTruthy();
	});

	it('orders the list newest first, breaking a same-day tie by version', async () => {
		stubFetch();
		render(ChangelogPage);

		await screen.findByText('Skills catalog');
		const text = document.body.textContent ?? '';

		// v0.2.0 and v0.3.0 share 2026-09-19, so the version decides which comes first.
		expect(text.indexOf('v0.4.0')).toBeLessThan(text.indexOf('v0.3.0'));
		expect(text.indexOf('v0.3.0')).toBeLessThan(text.indexOf('v0.2.0'));
		expect(text.indexOf('v0.2.0')).toBeLessThan(text.indexOf('v0.1.0'));
	});

	it('marks every release against the running build', async () => {
		stubFetch({ version: { ...VERSION, version: 'v0.3.0' } });
		render(ChangelogPage);

		expect(await screen.findByText('1 release newer than this build.')).toBeTruthy();
		expect(within(rowFor('v0.4.0')).getByText('Newer')).toBeTruthy();
		expect(within(rowFor('v0.3.0')).getByText('Running')).toBeTruthy();
		expect(within(rowFor('v0.2.0')).getByText('Installed')).toBeTruthy();
		expect(within(rowFor('v0.1.0')).getByText('Installed')).toBeTruthy();
	});

	it('says this build is the newest release listed when nothing is newer', async () => {
		stubFetch();
		render(ChangelogPage);

		expect(await screen.findByText('This build is the newest release listed.')).toBeTruthy();
		expect(within(rowFor('v0.4.0')).getByText('Running')).toBeTruthy();
	});

	it('counts a release newer than a prerelease build, which is what a dev binary sees', async () => {
		stubFetch({ version: { ...VERSION, version: '0.1.0-dev' } });
		render(ChangelogPage);

		expect(await screen.findByText('4 releases newer than this build.')).toBeTruthy();
		expect(within(rowFor('v0.1.0')).getByText('Newer')).toBeTruthy();
	});
});

describe('ChangelogPage version failures', () => {
	it('still renders the releases when the version read failed, and names the cause', async () => {
		stubFetch({
			version: { error: { code: 'INTERNAL_ERROR', message: 'The version route is down.' } },
			versionStatus: 500
		});
		render(ChangelogPage);

		expect(
			await screen.findByText('Could not read the running version from the gateway.')
		).toBeTruthy();
		expect(screen.getByText('The version route is down.')).toBeTruthy();
		expect(screen.getByText('Skills catalog')).toBeTruthy();
		expect(screen.queryByText('Newer')).toBeNull();
		expect(screen.queryByText('Installed')).toBeNull();
	});

	it('says a version that is not a release number cannot be compared', async () => {
		stubFetch({ version: { ...VERSION, version: 'dev-build' } });
		render(ChangelogPage);

		expect(await screen.findByText(/reports its version as dev-build/)).toBeTruthy();
		expect(screen.queryByText('Newer')).toBeNull();
		expect(screen.queryByText('Installed')).toBeNull();
		// The list stays on screen: the releases are worth reading without a comparison.
		expect(screen.getAllByRole('listitem').length).toBe(4);
	});
});
