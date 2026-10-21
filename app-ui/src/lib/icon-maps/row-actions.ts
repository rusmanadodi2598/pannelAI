// The row actions of the Endpoint & Key tables and the provider detail screens.
//
// Every button that uses one of these is icon-only, so the button's own `aria-label` carries the action's
// name and the glyph stays decorative (R-04, R-31). The verb reasons read "row" rather than "key" because
// the same actions serve the provider screens: a disabled row is a key or a model, and the glyph means the
// same thing on both.
import {
	Activity,
	Check,
	ChevronDown,
	ChevronUp,
	Pause,
	Pencil,
	Play,
	Trash2,
	X
} from '@lucide/svelte';

export const ROW_ACTION_ICONS = {
	rename: {
		icon: Pencil,
		reason: 'Renaming edits the stored name in place, which is what a pencil marks.'
	},
	moveUp: {
		icon: ChevronUp,
		reason:
			'Moving an entry up rewrites the order, so the upward chevron marks one step toward the front.'
	},
	moveDown: {
		icon: ChevronDown,
		reason: 'Moving an entry down rewrites the order, the same step read toward the back.'
	},
	edit: {
		icon: Pencil,
		reason: 'Editing opens the form that changes the stored fields of a row or a node.'
	},
	disable: {
		icon: Pause,
		reason:
			'Disabling pauses a row without destroying it: the gateway stops spending the credential or routing the model, which is what pause means.'
	},
	enable: {
		icon: Play,
		reason: 'Enabling resumes a row that was paused, the same control read the other way.'
	},
	test: {
		icon: Activity,
		reason: 'A test asks the upstream to answer once and reports whether it did, a liveness probe.'
	},
	delete: {
		icon: Trash2,
		reason: 'Deleting takes the row off the screen for good, which is what the bin marks.'
	},
	remove: {
		icon: Trash2,
		reason: 'Removing takes a declared custom model out of the catalog.'
	},
	save: {
		icon: Check,
		reason: 'Saving commits the edit the form or the row holds.'
	},
	cancel: {
		icon: X,
		reason: 'Cancelling leaves the stored values as they were.'
	}
} as const;

export type RowAction = keyof typeof ROW_ACTION_ICONS;
