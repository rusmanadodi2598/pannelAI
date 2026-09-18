// Error view test.
//
// SPEC-UI §5.1 lists the unknown-path view as U0: it has to name the requested path and offer the way
// back to /endpoint-keys. The copy is chosen by status, so this walks the statuses the panel can explain
// plus one it cannot, which is what the fallback exists for. It also pins the two things a reader would
// otherwise have to trust: the recovery link is always present, and the requested path appears for a 404
// only, so a 500 cannot leak an internal address into the page.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ErrorPage from '../../src/routes/+error.svelte';

// A mutable page stand-in. `vi.hoisted` lifts it above the mock factory, which is hoisted to the top of
// the file, so the factory and the tests share one object.
const pageState = vi.hoisted(() => ({
	status: 404,
	url: { pathname: '/' }
}));

vi.mock('$app/state', () => ({ page: pageState }));
vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

type Case = {
	status: number;
	pathname: string;
	heading: string;
	showsPath: boolean;
	why: string;
};

const CASES: Case[] = [
	{
		status: 404,
		pathname: '/does-not-exist',
		heading: 'No screen is routed here',
		showsPath: true,
		why: 'a route that is not built'
	},
	{
		status: 403,
		pathname: '/forbidden',
		heading: 'This screen is not available to you',
		showsPath: false,
		why: 'a refused request'
	},
	{
		status: 500,
		pathname: '/broken',
		heading: 'The panel could not load this screen',
		showsPath: false,
		why: 'a server failure'
	},
	{
		status: 418,
		pathname: '/teapot',
		heading: 'The panel could not load this screen',
		showsPath: false,
		why: 'a status the table does not name falls back'
	}
];

describe('error view', () => {
	afterEach(cleanup);

	for (const testCase of CASES) {
		it(`explains status ${testCase.status} (${testCase.why})`, () => {
			pageState.status = testCase.status;
			pageState.url.pathname = testCase.pathname;

			render(ErrorPage);

			expect(screen.getByRole('heading', { name: testCase.heading })).toBeTruthy();
		});
	}

	it('always offers the way back to endpoint keys', () => {
		pageState.status = 404;
		pageState.url.pathname = '/nope';

		render(ErrorPage);

		const link = screen.getByRole('link', { name: 'Back to endpoint keys' });
		expect(link.getAttribute('href')).toBe('/endpoint-keys');
	});

	it('names the requested path for a 404', () => {
		pageState.status = 404;
		pageState.url.pathname = '/a-very-long-address-that-is-not-routed-anywhere-in-the-panel';

		render(ErrorPage);

		expect(
			screen.getByText('/a-very-long-address-that-is-not-routed-anywhere-in-the-panel')
		).toBeTruthy();
	});

	it('does not name the requested path for a status other than 404', () => {
		pageState.status = 500;
		pageState.url.pathname = '/internal/address';

		render(ErrorPage);

		expect(screen.queryByText('Requested path')).toBeNull();
		expect(screen.queryByText('/internal/address')).toBeNull();
	});
});
