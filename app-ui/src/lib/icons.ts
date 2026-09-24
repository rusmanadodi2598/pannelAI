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
// Three maps live here: the sidebar's rows, the section headers that are not nav rows, and the row
// actions of the Endpoint & Key tables. The third is what makes an icon-only button legible: the glyph
// is decorative, the reason is recorded, and the button itself carries the accessible name.

import {
	Activity,
	Ban,
	Blocks,
	BookOpen,
	Binary,
	ChartLine,
	Check,
	CircleGauge,
	Film,
	History,
	Image,
	KeyRound,
	Layers,
	MessageSquareCode,
	Mic,
	Network,
	Pause,
	Pencil,
	Play,
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
 * The row actions of the Endpoint & Key tables.
 *
 * Every button that uses one of these is icon-only, so the button's own `aria-label` carries the
 * action's name and the glyph stays decorative. The reasons say what each shape has to communicate,
 * which is the part a reviewer checks (R-04, R-31).
 */
export const ROW_ACTION_ICONS = {
	rename: {
		icon: Pencil,
		reason: 'Renaming edits the stored name in place, which is what a pencil marks.'
	},
	disable: {
		icon: Pause,
		reason:
			'Disabling stops a credential from being spent without destroying it, which is what pause means.'
	},
	enable: {
		icon: Play,
		reason: 'Enabling resumes a credential that was paused, the same control read the other way.'
	},
	revoke: {
		icon: Ban,
		reason: 'Revoking is terminal, so a prohibition mark says the key can never be used again.'
	},
	test: {
		icon: Activity,
		reason: 'A test asks the upstream to answer once and reports whether it did, a liveness probe.'
	},
	delete: {
		icon: Trash2,
		reason: 'Deleting removes the stored key row itself, which is what the bin marks.'
	},
	save: {
		icon: Check,
		reason: 'Saving commits the name being edited in the row.'
	},
	cancel: {
		icon: X,
		reason: 'Cancelling leaves the stored name as it was.'
	}
} as const;

export type RowAction = keyof typeof ROW_ACTION_ICONS;

export function navIcon(key: string): IconEntry | undefined {
	return NAV_ICONS[key];
}

export function rowActionIcon(action: RowAction): IconEntry {
	return ROW_ACTION_ICONS[action];
}
