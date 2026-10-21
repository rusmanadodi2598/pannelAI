// The controls that are not row actions: copy, search, refresh, paging, folds, and bulk selection.
//
// The copy control is the oldest of these and carries no visible label in the modals, so its name lives in
// the button's `aria-label` and the glyph stays decorative, the same contract the row actions follow
// (R-04, R-31). The rest keep a visible label and gain the glyph beside it, which is the reference's own
// button shape (`providers/[id]/page.js` renders every action as icon + label).
import {
	Check,
	ChevronDown,
	ChevronLeft,
	ChevronRight,
	ChevronsUpDown,
	Copy,
	Download,
	KeyRound,
	ListPlus,
	Pause,
	Play,
	Plus,
	RefreshCw,
	Search,
	Square,
	SquareCheckBig,
	Tag,
	X
} from '@lucide/svelte';

export const CONTROL_ICONS = {
	copy: {
		icon: Copy,
		reason: 'Copying puts the value on the clipboard, which is what the two stacked sheets mark.'
	},
	search: {
		icon: Search,
		reason: 'Search asks the list for the rows matching what was typed.'
	},
	refresh: {
		icon: RefreshCw,
		reason: 'Refreshing re-reads the same set, which the circular arrow marks.'
	},
	previous: {
		icon: ChevronLeft,
		reason: 'Previous moves one step back through an ordered, paged set.'
	},
	next: {
		icon: ChevronRight,
		reason: 'Next moves one step forward through an ordered, paged set.'
	},
	add: {
		icon: Plus,
		reason: 'Adding starts one more of the kind the control sits beside.'
	},
	addMany: {
		icon: ListPlus,
		reason:
			'Adding several at once stores many rows from one paste, which the list-plus glyph marks.'
	},
	addKey: {
		icon: KeyRound,
		reason: 'Adding a key stores one more credential the gateway can spend for this provider.'
	},
	import: {
		icon: Download,
		reason: 'Importing pulls the model list that the node itself answers.'
	},
	open: {
		icon: ChevronRight,
		reason: 'Opening a row moves one step into the screen that row owns.'
	},
	clear: {
		icon: X,
		reason: 'Clearing empties the filters, which the cross marks.'
	},
	choose: {
		icon: ChevronsUpDown,
		reason: 'Choosing opens the list the value is picked from, which the stacked chevrons mark.'
	},
	done: {
		icon: Check,
		reason: 'Done confirms the choices already made and closes the picker.'
	},
	fold: {
		icon: ChevronDown,
		reason:
			'Folding tucks a card body away behind a header that already counts what is inside; the chevron marks the tucked edge and turns when the body opens.'
	},
	select: {
		icon: SquareCheckBig,
		reason:
			'Select all marks every row the server counted, not only the page on screen, so the checked box names the wider set.'
	},
	selectNone: {
		icon: Square,
		reason:
			'Clearing the selection empties an already-checked box, the same control read the other way.'
	},
	tag: {
		icon: Tag,
		reason:
			'A tag is the label a bulk action is addressed to, and the price tag is the shape that means a label you select by.'
	},
	pause: {
		icon: Pause,
		reason:
			'Pausing stops the auto-refresh timer, holding the screen on the last read the operator chose to keep.'
	},
	resume: {
		icon: Play,
		reason:
			'Resuming restarts the auto-refresh timer the operator paused, the same control read the other way.'
	},
	save: {
		icon: Check,
		reason: 'Saving commits the form draft into the stored document the gateway serves.'
	}
} as const;
