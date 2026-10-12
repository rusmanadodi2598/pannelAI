// The provider detail screens fold their sections, page their long lists, and keep their forms symmetric
// (antislop-ui "Editor That Sits Above the List" sibling rules, antislop-layoutmobile Form Row).
//
// Measured on the running screen before the change: the openai catalog rendered 36 rows in one table and
// the page ran past 16,000px on a phone. After it: every section is one closed row, and an opened table
// shows ten rows with Page 1 of 4.
//
// The page-arithmetic cases are table-driven so the clamp is checked where the list shrinks too.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import SectionFold from '../../src/lib/components/SectionFold.svelte';
import { createRawSnippet } from 'svelte';
import { pageOf, ROWS_PER_PAGE } from '$lib/schemas/local-page';

afterEach(() => {
	cleanup();
});

describe('pageOf', () => {
	const cases = [
		{
			name: 'empty list is one empty page',
			items: [] as number[],
			page: 1,
			rows: 0,
			pages: 1,
			show: 1
		},
		{ name: 'exactly one page', items: [1, 2, 3], page: 1, rows: 3, pages: 1, show: 1 },
		{
			name: 'ten rows is exactly one page',
			items: Array.from({ length: 10 }, (_, i) => i),
			page: 1,
			rows: 10,
			pages: 1,
			show: 1
		},
		{
			name: 'eleven rows is two pages, ten then one',
			items: Array.from({ length: 11 }, (_, i) => i),
			page: 2,
			rows: 1,
			pages: 2,
			show: 2
		},
		{
			name: 'thirty six models is four pages',
			items: Array.from({ length: 36 }, (_, i) => i),
			page: 1,
			rows: 10,
			pages: 4,
			show: 1
		},
		{
			name: 'a page past the end clamps to the last',
			items: Array.from({ length: 36 }, (_, i) => i),
			page: 99,
			rows: 6,
			pages: 4,
			show: 4
		},
		{
			name: 'a page below one clamps to the first',
			items: Array.from({ length: 36 }, (_, i) => i),
			page: 0,
			rows: 10,
			pages: 4,
			show: 1
		}
	];

	it.each(cases)('$name', ({ items, page, rows, pages, show }) => {
		const view = pageOf(items, page);
		expect(view.rows.length).toBe(rows);
		expect(view.pages).toBe(pages);
		expect(view.page).toBe(show);
		expect(view.total).toBe(items.length);
	});

	it('never renders more than the row cap', () => {
		const view = pageOf(
			Array.from({ length: 500 }, (_, i) => i),
			1
		);
		expect(view.rows.length).toBeLessThanOrEqual(ROWS_PER_PAGE);
	});
});

describe('SectionFold', () => {
	const body = () => createRawSnippet(() => ({ render: () => '<p>inner row</p>' }));

	it('starts closed, hides its body, and opens on the heading button', async () => {
		render(SectionFold, { props: { title: 'Model catalog', children: body() } });

		const heading = screen.getByRole('button', { name: /Model catalog/ });
		expect(heading.getAttribute('aria-expanded')).toBe('false');
		expect(screen.queryByText('inner row')).toBeNull();

		await fireEvent.click(heading);
		expect(heading.getAttribute('aria-expanded')).toBe('true');
		expect(screen.getByText('inner row')).toBeTruthy();
	});

	it('starts open when asked, and closes again', async () => {
		render(SectionFold, { props: { title: 'Connections', open: true, children: body() } });

		const heading = screen.getByRole('button', { name: /Connections/ });
		expect(heading.getAttribute('aria-expanded')).toBe('true');

		await fireEvent.click(heading);
		expect(heading.getAttribute('aria-expanded')).toBe('false');
	});

	it('shows the summary beside the title while closed', () => {
		render(SectionFold, {
			props: { title: 'Connections', summary: '3 connections', children: body() }
		});
		expect(
			within(screen.getByRole('button', { name: /Connections/ })).getByText('3 connections')
		).toBeTruthy();
	});
});
