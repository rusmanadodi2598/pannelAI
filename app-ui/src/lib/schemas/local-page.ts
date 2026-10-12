// The local page of a table the server does not page (docs/SPEC-UI/001-SPEC-UI.md §6.3, antislop-ui
// "Flat List With No Density Control").
//
// A provider can carry a few hundred models and a connection can carry 100+ keys, so a table that renders
// every row makes the screen unreadable on a phone. The rule is one page of at most ten rows, with
// Previous and Next beside the table, and the page number clamped whenever the list shrinks.

export const ROWS_PER_PAGE = 10;

export type Page<T> = {
	rows: T[];
	page: number;
	pages: number;
	total: number;
};

/** Clamps a 1-based page into the range the list allows, and returns the slice that page shows. */
export function pageOf<T>(items: readonly T[], requested: number, size = ROWS_PER_PAGE): Page<T> {
	const total = items.length;
	const pages = Math.max(1, Math.ceil(total / size));
	const page = Math.min(Math.max(1, Math.floor(requested) || 1), pages);
	const start = (page - 1) * size;
	return { rows: items.slice(start, start + size), page, pages, total };
}
