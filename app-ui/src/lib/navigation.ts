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

export type NavNode = {
	key: string;
	label: string;
	/**
	 * Present only when the screen exists. Typed as a RouteId so a typo cannot compile, and absent for
	 * a planned screen so the renderer has nothing to link to.
	 */
	href?: RouteId;
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
			{ key: 'providers', label: 'Provider', planned: true },
			{ key: 'combos', label: 'Combo & Vision Adapter', planned: true },
			{ key: 'media-providers', label: 'Media Provider', planned: true, children: [] }
		]
	},
	{
		key: 'observe',
		label: 'Observe',
		reason: 'Answers what already happened: consumption, remaining quota, and runtime output.',
		items: [
			{ key: 'usage', label: 'Usage', planned: true },
			{ key: 'quota', label: 'Quota Tracker', planned: true },
			{ key: 'console-log', label: 'Console Log', planned: true }
		]
	},
	{
		key: 'optimize',
		label: 'Optimize',
		reason: 'Answers what can be cheaper or reused: token reduction and client-side skills.',
		items: [
			{ key: 'token-saver', label: 'Token Saver', planned: true },
			{ key: 'skills', label: 'Skill', planned: true }
		]
	},
	{
		key: 'developer',
		label: 'Developer',
		reason: 'Answers how a client talks to the gateway: try it, read the contract, track changes.',
		items: [
			{ key: 'playground', label: 'Playground Chat', planned: true },
			{ key: 'api-docs', label: 'API Docs', planned: true },
			{ key: 'changelog', label: 'Changelog', planned: true }
		]
	},
	{
		key: 'system',
		label: 'System',
		reason: 'Answers how the panel itself behaves: outbound routing and panel configuration.',
		items: [
			{ key: 'proxies', label: 'Proxy Pools', planned: true },
			{ key: 'settings', label: 'Setting', href: '/settings' }
		]
	}
];

// The Media kinds live under one container so the sidebar does not carry six extra top-level rows.
// The container itself is not a screen, so it carries no href; `planned` is set on it only to keep the
// shape uniform, and the renderer treats a node with children as a disclosure instead.
// `web` maps to the API kind `search` (SPEC-UI §6.8), and the label is "Web Search" because no fetch
// endpoint exists, which is the one place a label had to match the API rather than the legacy panel.
const MEDIA_KINDS: NavNode[] = [
	{ key: 'media-embedding', label: 'Embedding', planned: true },
	{ key: 'media-image', label: 'Image', planned: true },
	{ key: 'media-video', label: 'Video', planned: true },
	{ key: 'media-tts', label: 'TTS', planned: true },
	{ key: 'media-stt', label: 'STT', planned: true },
	{ key: 'media-search', label: 'Web Search', planned: true }
];

// Filled here rather than inline so the container above stays readable and the kinds have one home.
const mediaGroup = NAV_GROUPS.find((group) => group.key === 'configure')?.items.find(
	(item) => item.key === 'media-providers'
);
if (mediaGroup) mediaGroup.children = MEDIA_KINDS;

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
