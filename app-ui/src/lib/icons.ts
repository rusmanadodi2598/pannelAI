// Navigation icon map (docs/SPEC-UI/001-SPEC-UI.md §8.11).
//
// One entry per navigation item, each with a written reason, because R-04 rejects an icon set chosen
// for its library look and R-31 requires the purpose to be writable in one line. Identifier names
// come from lucide-svelte, the set the component layer ships with; a name that does not resolve is a
// build error rather than a blank space in the sidebar.

import {
	Blocks,
	BookOpen,
	Binary,
	CircleGauge,
	Film,
	Image,
	KeyRound,
	ChartLine,
	Layers,
	Mic,
	Network,
	Scissors,
	ScrollText,
	Search,
	Server,
	Settings,
	Volume2
} from 'lucide-svelte';

// The type of a lucide icon, inferred from one of them. Every icon in the set shares this signature,
// and inferring it here avoids restating the library's generics incorrectly.
export type NavIcon = {
	icon: typeof KeyRound;
	reason: string;
};

export const NAV_ICONS: Record<string, NavIcon> = {
	'endpoint-keys': {
		icon: KeyRound,
		reason: 'The screen is about keys, client-facing and upstream.'
	},
	providers: { icon: Server, reason: 'A provider is a remote host the gateway calls.' },
	combos: { icon: Layers, reason: 'A combo is an ordered list of models tried in sequence.' },
	usage: { icon: ChartLine, reason: 'Usage is a time series, not a single number.' },
	quota: { icon: CircleGauge, reason: 'Quota is headroom against a cap, which a gauge shows.' },
	'token-saver': {
		icon: Scissors,
		reason: 'The savers cut tokens out of a request before sending.'
	},
	'media-embedding': { icon: Binary, reason: 'Embeddings are numeric vectors.' },
	'media-image': { icon: Image, reason: 'Direct match to the image generation kind.' },
	'media-video': { icon: Film, reason: 'Direct match to the video generation kind.' },
	'media-tts': { icon: Volume2, reason: 'The text-to-speech provider produces audio.' },
	'media-stt': { icon: Mic, reason: 'The speech-to-text provider consumes audio.' },
	'media-search': { icon: Search, reason: 'The search kind is a query against the web.' },
	proxies: { icon: Network, reason: 'Proxies are intermediate hops in the outbound path.' },
	skills: { icon: Blocks, reason: 'A skill is a reusable instruction block handed to a client.' },
	logs: { icon: ScrollText, reason: 'Logs are records read after the fact.' },
	'api-docs': { icon: BookOpen, reason: 'The screen is documentation, not an action.' },
	settings: { icon: Settings, reason: 'Direct match to the settings screen.' }
};

export function navIcon(key: string): NavIcon | undefined {
	return NAV_ICONS[key];
}
