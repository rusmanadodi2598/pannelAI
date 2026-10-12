// Test helper: open the provider detail sections before asserting on what they hold.
//
// The provider detail screen folds each section (antislop-ui, Flat List With No Density Control). A closed
// section does not render its body, so a test that reaches for a table or a control has to open the section
// the way an operator does: by pressing its heading. Opening every section keeps each assertion exactly what
// it was, because the operator who needs a table presses the heading to get it.

import { fireEvent, screen, waitFor } from '@testing-library/svelte';

/** Opens the fold whose heading starts with `title`, if it is closed. Returns the heading button. */
export async function openSection(title: string): Promise<HTMLElement | null> {
	const heading = screen.queryByRole('button', { name: new RegExp(`^${title}`) });
	if (heading && heading.getAttribute('aria-expanded') === 'false') {
		await fireEvent.click(heading);
	}
	return heading;
}

/**
 * Opens the named tab on a screen that groups its content into tabs (the provider detail).
 *
 * The tab bar renders before the panel, so a test that needs a panel's content switches to it the way an
 * operator does. A screen with no tab bar is left alone, so this is safe to call from either shape.
 */
export async function openTab(label: string): Promise<void> {
	const tab = screen.queryByRole('tab', { name: new RegExp(`^${label}$`) });
	if (tab && tab.getAttribute('aria-selected') !== 'true') {
		await fireEvent.click(tab);
	}
}

/**
 * Opens every tab, then every folded section inside each, so a test that asserts on any section's content
 * reads it the way an operator reaches it. Used where a case spans more than one tab.
 */
export async function openEveryTab(): Promise<void> {
	const tabs = screen.queryAllByRole('tab').filter((t) => t.closest('[role=tablist]') !== null);
	for (const tab of tabs) {
		if (tab.getAttribute('aria-selected') !== 'true') await fireEvent.click(tab);
	}
}

/** Opens every folded section on the screen, in order. A section that is already open is left as it is. */
export async function openAllSections(tabLabel?: string): Promise<void> {
	// A tab panel renders only while its tab is selected, so a case that asserts on one tab's content names it
	// here and gets that panel open with its folds. Without a label every tab is visited in turn, which is what
	// a case spanning the whole screen needs.
	if (tabLabel !== undefined) {
		// The page loads its provider after mount, so the tab bar appears a tick later.
		await waitFor(() => {
			if (screen.queryByRole('tab', { name: new RegExp(`^${tabLabel}$`) }) === null) {
				throw new Error(`the ${tabLabel} tab is not on screen yet`);
			}
		});
		await openTab(tabLabel);
		// The panel is up once its tab is selected. Some tabs hold facts rather than folds (the registry's
		// Overview), so the panel itself is what there is to wait for, not a fold.
		await waitFor(() => {
			const selected = screen.queryByRole('tab', { name: new RegExp(`^${tabLabel}$`) });
			if (selected?.getAttribute('aria-selected') !== 'true') {
				throw new Error(`the ${tabLabel} tab is not selected yet`);
			}
		});
		await openFoldsInDom();
		return;
	}

	// The detail page groups its sections into tabs. Every fold that matters sits under some tab, so each tab
	// is opened in turn and the folds inside it are opened too: opening only the first tab's folds leaves the
	// rest unrendered, and a case that reads a later tab never sees it.
	const tabs = screen.queryAllByRole('tab').filter((t) => t.closest('[role=tablist]') !== null);
	const labels = tabs.length > 0 ? tabs : [null];

	// Wait for the page to draw at least one fold before opening anything, so a slow read does not race this.
	await waitFor(() => {
		if (!screen.queryAllByRole('button').some((b) => b.hasAttribute('aria-expanded'))) {
			throw new Error('provider detail folds not rendered yet');
		}
	});

	for (const tab of labels) {
		if (tab === null) {
			await openFoldsInDom();
			continue;
		}
		if (tab.getAttribute('aria-selected') !== 'true') {
			await fireEvent.click(tab);
			// The panel after a tab switch is rendered on the next tick.
			await waitFor(() => {
				if (!screen.queryAllByRole('button').some((b) => b.hasAttribute('aria-expanded'))) {
					throw new Error('the tab panel has no sections yet');
				}
			});
		}
		await openFoldsInDom();
	}
}

/** Opens every closed fold currently on the screen. */
async function openFoldsInDom(): Promise<void> {
	for (const heading of screen
		.queryAllByRole('button')
		.filter((b) => b.hasAttribute('aria-expanded'))) {
		if (heading.getAttribute('aria-expanded') === 'false') await fireEvent.click(heading);
	}
}
