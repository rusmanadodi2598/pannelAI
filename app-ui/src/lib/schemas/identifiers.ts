// The identifiers the API mints, and how the panel recognises one.
//
// These are not field classes, which is what `./primitives` holds one of each: an identifier is a string
// with a resource prefix (`ep_`, `uky_`, `gky_`, `prx_`), and a read that carries one is checked against
// the prefix its own resource uses, so a mixed-up response fails at the boundary instead of rendering
// another resource's row. Split out so `./primitives` keeps room under the panel's line limit.

import { z } from 'zod';

export function prefixedId(prefix: string) {
	return z
		.string()
		.refine((value) => value.startsWith(prefix), { message: `Expected an ${prefix} identifier.` });
}

export const gatewayKeyId = prefixedId('gky_');

// The upstream identifiers the API mints (SPEC-API §7.5).
export const endpointId = prefixedId('ep_');
export const upstreamKeyId = prefixedId('uky_');
