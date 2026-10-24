// Icon maps (docs/SPEC-UI/001-SPEC-UI.md §8.11, DESIGN.md §10).
//
// One entry per icon, each with a written reason: R-04 rejects a set chosen for its library look, and R-31
// requires the purpose to be writable in one line. The maps live in `src/lib/icon-maps/`, one file per place
// a glyph is used, so a module evaluates only the glyphs it names. This file is the barrel every screen
// imports, which is why the four subjects could move without touching a call site.
import { NAV_ICONS } from './icon-maps/nav';
import { ROW_ACTION_ICONS, type RowAction } from './icon-maps/row-actions';
import type { IconEntry } from './icon-maps/types';

export { NAV_ICONS, STATUS_ICONS } from './icon-maps/nav';
export { ROW_ACTION_ICONS, type RowAction } from './icon-maps/row-actions';
export { CONTROL_ICONS } from './icon-maps/controls';
export type { IconEntry } from './icon-maps/types';

export function navIcon(key: string): IconEntry | undefined {
	return NAV_ICONS[key];
}

export function rowActionIcon(action: RowAction): IconEntry {
	return ROW_ACTION_ICONS[action];
}
