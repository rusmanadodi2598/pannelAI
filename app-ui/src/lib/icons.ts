// Icon maps (docs/SPEC-UI/001-SPEC-UI.md §8.11, DESIGN.md §10).
//
// One entry per icon, each with a written reason, because R-04 rejects an icon set chosen for its
// library look and R-31 requires the purpose to be writable in one line. Identifier names come from
// `@lucide/svelte`, the set the component layer ships with; a name that does not resolve is a build
// error rather than a blank space in the sidebar.
//
// The set is one library on purpose. R-04 warns that a single installed set gives every panel the same
// thin-stroke look, which is why every row below carries a reason that ties the glyph to its content
// rather than to the library: the relevance is the decision, and the set is the implementation.
//
// Four maps live here: the sidebar's rows, the section headers that are not nav rows, the row actions
// of the Endpoint & Key tables, and the controls that are neither. The last two are what make an
// icon-only button legible: the glyph is decorative, the reason is recorded, and the button itself
// carries the accessible name.

import {
	Activity,
	Blocks,
	BookOpen,
	Binary,
	ChartLine,
	Check,
	ChevronDown,
	ChevronLeft,
	ChevronRight,
	ChevronUp,
	ChevronsUpDown,
	CircleGauge,
	Copy,
	Download,
	Film,
	History,
	Image,
	KeyRound,
	Layers,
	ListPlus,
	MessageSquareCode,
	Mic,
	Network,
	Pause,
	Pencil,
	Play,
	Plus,
	RefreshCw,
	Scissors,
	ScrollText,
	Search,
	Server,
	Settings,
	TerminalSquare,
	Trash2,
	Volume2,
	X
} from '@lucide/svelte';

// The type of a lucide icon, inferred from one of them. Every icon in the set shares this signature,
// and inferring it here avoids restating the library's generics incorrectly.
export type IconEntry = {
	icon: typeof KeyRound;
	reason: string;
};

export const NAV_ICONS: Record<string, IconEntry> = {
	'endpoint-keys': {
		icon: KeyRound,
		reason: 'The screen is about keys, client-facing and upstream.'
	},
	providers: { icon: Server, reason: 'A provider is a remote host the gateway calls.' },
	combos: { icon: Layers, reason: 'A combo is an ordered list of models tried in sequence.' },
	'media-providers': {
		icon: Image,
		reason: 'The group covers non-text models, and image is the kind operators configure first.'
	},
	'media-embedding': { icon: Binary, reason: 'Embeddings are numeric vectors.' },
	'media-image': { icon: Image, reason: 'Direct match to the image generation kind.' },
	'media-video': { icon: Film, reason: 'Direct match to the video generation kind.' },
	'media-tts': { icon: Volume2, reason: 'The text-to-speech provider produces audio.' },
	'media-stt': { icon: Mic, reason: 'The speech-to-text provider consumes audio.' },
	'media-search': { icon: Search, reason: 'The search kind is a query against the web.' },
	usage: { icon: ChartLine, reason: 'Usage is a time series, not a single number.' },
	quota: { icon: CircleGauge, reason: 'Quota is headroom against a cap, which a gauge shows.' },
	'console-log': {
		icon: TerminalSquare,
		reason: 'The console buffer is terminal output the gateway captured, not a written audit trail.'
	},
	'token-saver': {
		icon: Scissors,
		reason: 'The savers cut tokens out of a request before sending.'
	},
	skills: { icon: Blocks, reason: 'A skill is a reusable instruction block handed to a client.' },
	playground: {
		icon: MessageSquareCode,
		reason: 'The screen sends a chat request by hand to see what the gateway answers.'
	},
	'api-docs': { icon: BookOpen, reason: 'The screen is documentation, not an action.' },
	changelog: {
		icon: History,
		reason: 'A changelog is a record ordered by time, which is what the history glyph marks.'
	},
	proxies: { icon: Network, reason: 'Proxies are intermediate hops in the outbound path.' },
	settings: { icon: Settings, reason: 'Direct match to the settings screen.' }
};

/** Kept for the screens that are not navigation rows. Re-exported so one module owns every icon name. */
export const STATUS_ICONS = {
	'logs-requests': {
		icon: ScrollText,
		reason: 'Request logs are written records read after the fact.'
	}
} as const;

/**
 * The row actions of the Endpoint & Key tables and the provider detail screens.
 *
 * Every button that uses one of these is icon-only, so the button's own `aria-label` carries the
 * action's name and the glyph stays decorative. The reasons say what each shape has to communicate,
 * which is the part a reviewer checks (R-04, R-31). The verb reasons read "row" rather than "key"
 * because the same actions serve the provider screens: a disabled row is a key or a model, and the
 * glyph means the same thing on both.
 */
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

/**
 * The controls that are not row actions.
 *
 * The copy control is the oldest of these: it carries no visible label in the modals, so its name lives
 * in the button's `aria-label` and the glyph stays decorative, the same contract the row actions follow
 * (R-04, R-31). The rest are the provider screens' toolbar and navigation controls; those keep a visible
 * label and gain the glyph beside it, which is the reference's own button shape (`providers/[id]/page.js`
 * renders every action as icon + label).
 */
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

export function navIcon(key: string): IconEntry | undefined {
	return NAV_ICONS[key];
}

export function rowActionIcon(action: RowAction): IconEntry {
	return ROW_ACTION_ICONS[action];
}
