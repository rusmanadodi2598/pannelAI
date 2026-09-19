// A reactive stand-in for SvelteKit's `page`, for component tests that read the URL.
//
// The Usage and Quota screens treat the URL as the source of truth (SPEC-UI §8.4.2), so a test that wants
// to see a control change the view has to let the change actually reach `page.url` and re-trigger the
// component's effect. A plain object would not: Svelte tracks reads, and only a `$state` proxy notifies.
// That is why this is a `.svelte.ts` module rather than a plain helper, and why the `$app/navigation` mock
// writes here instead of recording calls.
//
// The `goto` mock in each test file parses the URL it was handed, exactly as the router would, so the
// assertion is on the URL the panel produced rather than on the fact that it called something.

import { SvelteURLSearchParams } from 'svelte/reactivity';

export const pageState = $state({
	url: {
		pathname: '/usage',
		searchParams: new SvelteURLSearchParams()
	}
});

/** Points the stand-in page at a path and query, the way a navigation would. */
export function visit(pathname: string, query = ''): void {
	pageState.url = { pathname, searchParams: new SvelteURLSearchParams(query) };
}

/** The query half of a URL the panel asked to navigate to. */
export function queryOf(url: string): string {
	const index = url.indexOf('?');
	return index === -1 ? '' : url.slice(index + 1);
}
