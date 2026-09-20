// The address a navigation row links to.
//
// One definition, because two components need it and they have to agree: the row builds the link, and
// the sidebar decides whether a container starts open. A row that links somewhere the sidebar does not
// recognise would leave a Media kind page with its own row hidden inside a closed disclosure, which is
// exactly the bug this file exists to prevent.
//
// `resolve` lives here rather than in `navigation.ts` so that file stays data plus one path predicate,
// importable by a test that has no router to resolve against.

import { resolve } from '$app/paths';
import type { NavNode } from './navigation';

/**
 * The resolved path for a row, or undefined when the row links nowhere.
 *
 * A static route and a parameterised one both come back as an address, so a caller never has to know
 * which shape the row used. That is what lets the active check and the disclosure check read the same
 * value as the `href` attribute.
 */
export function navPath(node: NavNode): string | undefined {
	if (node.href !== undefined) return resolve(node.href);
	if (node.link !== undefined) return resolve(node.link.route, node.link.params);
	return undefined;
}
