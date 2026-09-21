// Panel navigation as data.
//
// The sidebar is generated from this file, so a screen is added in one place and the route, the icon,
// and the planned state stay consistent. Two rules drive the shape:
//
//   R-24: an item may only carry an `href` when a route file exists. A screen that is not built yet has
//   no `href` at all, which makes a dead link impossible to write rather than merely discouraged. The
//   route existence itself is asserted in tests/navigation/navigation.test.ts against the filesystem.
//
//   R-31: the groups answer the operator's questions, so the grouping has a reason. A flat list of
//   nineteen rows would be a list, not a structure.
//
// Screen scope and phases come from SPEC-UI §2.1 and §5.1; the group names are this panel's own.

import type { RouteId } from '$app/types';
import { MEDIA_KIND_LABELS, MEDIA_KINDS, type MediaKind } from '$lib/schemas/media-provider';

/**
 * The routes a navigation row may link to with a static path.
 *
 * A parameterised route is not one of them, and the reason is the parameter: a row can only link where
 * it knows the address, and a parameter that names a record (a provider id) is something the sidebar has
 * no way to know. `resolve` could not build the link either.
 *
 * The exclusion is a subtraction from the generated `RouteId` rather than a hand-written union, so a
 * renamed or removed route still fails to compile here. If a new parameterised route is added, this type
 * keeps rejecting it, which is the intended failure: it forces the decision instead of silently widening
 * what the sidebar may link to.
 */
export type NavHref = Exclude<RouteId, '/providers/[provider_id]' | '/media-providers/[kind]'>;

/**
 * A link to a parameterised route, with the parameter the row itself fixes.
 *
 * The media kinds are why this exists. `Embedding` and `TTS` are one screen at six addresses, and the row
 * *is* the choice: the kind is part of the row's identity rather than something an operator picks after
 * opening the screen, so the row knows the parameter and can build the link. `/providers/[provider_id]`
 * stays out for the opposite reason, which is why `NavHref` keeps excluding it.
 *
 * The route is a one-member union rather than any `RouteId`, so a second parameterised nav target is a
 * deliberate edit to this type rather than something that compiles by accident.
 */
export type NavLink = {
	route: '/media-providers/[kind]';
	params: { kind: MediaKind };
};

export type NavNode = {
	key: string;
	label: string;
	/**
	 * Present only when the screen exists at a static path. Typed as a static route so a typo cannot
	 * compile, and absent for a planned screen so the renderer has nothing to link to.
	 */
	href?: NavHref;
	/** Present only when the screen exists at a parameterised path and the row fixes the parameter. */
	link?: NavLink;
	/** True when the screen is not built yet. The sidebar renders a Planned chip for it. */
	planned?: boolean;
	/** Sub-items (the Media kinds). A node with children is a container and carries no href. */
	children?: NavNode[];
};

export type NavGroup = {
	key: string;
	label: string;
	/** Why this group exists, in one line. R-31 requires the reason to be writable. */
	reason: string;
	items: NavNode[];
};

export const NAV_GROUPS: NavGroup[] = [
	{
		key: 'configure',
		label: 'Configure',
		reason: 'Answers what the gateway routes to: keys, providers, combos, and media kinds.',
		items: [
			{ key: 'endpoint-keys', label: 'Endpoint & Key', href: '/endpoint-keys' },
			{ key: 'providers', label: 'Provider', href: '/providers' },
			{ key: 'combos', label: 'Combo & Vision Adapter', href: '/combos' },
			{ key: 'media-providers', label: 'Media Provider', planned: true, children: [] }
		]
	},
	{
		key: 'observe',
		label: 'Observe',
		reason: 'Answers what already happened: consumption, remaining quota, and runtime output.',
		items: [
			{ key: 'usage', label: 'Usage', href: '/usage' },
			{ key: 'quota', label: 'Quota Tracker', href: '/quota' },
			{ key: 'console-log', label: 'Console Log', href: '/console-log' }
		]
	},
	{
		key: 'optimize',
		label: 'Optimize',
		reason: 'Answers what can be cheaper or reused: token reduction and client-side skills.',
		items: [
			{ key: 'token-saver', label: 'Token Saver', href: '/token-saver' },
			{ key: 'skills', label: 'Skill', planned: true }
		]
	},
	{
		key: 'developer',
		label: 'Developer',
		reason: 'Answers how a client talks to the gateway: try it, read the contract, track changes.',
		items: [
			{ key: 'playground', label: 'Playground Chat', planned: true },
			{ key: 'api-docs', label: 'API Docs', href: '/api-docs' },
			{ key: 'changelog', label: 'Changelog', href: '/changelog' }
		]
	},
	{
		key: 'system',
		label: 'System',
		reason: 'Answers how the panel itself behaves: outbound routing and panel configuration.',
		items: [
			{ key: 'proxies', label: 'Proxy Pools', href: '/proxy-pools' },
			{ key: 'settings', label: 'Setting', href: '/settings' }
		]
	}
];

// The Media kinds live under one container so the sidebar does not carry six extra top-level rows.
// The container itself is not a screen, so it carries no href; `planned` is set on it only to keep the
// shape uniform, and the renderer treats a node with children as a disclosure instead.
//
// The keys, labels, and links are derived from the schema module's kind vocabulary rather than restated
// here, so a sidebar row and the heading of the page it opens cannot disagree. The label for `web` is
// "Web Search", which is the one place a label matches the API's meaning rather than the legacy panel's
// wording, because the panel has no fetch endpoint and "Web" alone would read as a browsing surface.
const MEDIA_KIND_NODE_KEYS: Record<MediaKind, string> = {
	embedding: 'media-embedding',
	image: 'media-image',
	video: 'media-video',
	tts: 'media-tts',
	stt: 'media-stt',
	web: 'media-search'
};

const MEDIA_KIND_NODES: NavNode[] = MEDIA_KINDS.map((kind) => ({
	key: MEDIA_KIND_NODE_KEYS[kind],
	label: MEDIA_KIND_LABELS[kind],
	link: { route: '/media-providers/[kind]', params: { kind } }
}));

// Filled here rather than inline so the container above stays readable and the kinds have one home.
const mediaGroup = NAV_GROUPS.find((group) => group.key === 'configure')?.items.find(
	(item) => item.key === 'media-providers'
);
if (mediaGroup) mediaGroup.children = MEDIA_KIND_NODES;

/**
 * True when `pathname` is the node's route or a child of it. Kept here rather than in the component so
 * the sidebar and any future breadcrumb agree on what "active" means.
 */
export function isActiveRoute(pathname: string, href: string): boolean {
	return pathname === href || pathname.startsWith(`${href}/`);
}

/** Every node in the tree, in render order. Used by the icon map test and by the renderer. */
export function allNodes(groups: NavGroup[] = NAV_GROUPS): NavNode[] {
	const nodes: NavNode[] = [];

	for (const group of groups) {
		const walk = (items: NavNode[]): void => {
			for (const item of items) {
				nodes.push(item);
				if (item.children) walk(item.children);
			}
		};
		walk(group.items);
	}

	return nodes;
}
