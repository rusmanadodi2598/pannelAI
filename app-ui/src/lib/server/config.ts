// Server configuration for the panel.
//
// Validated once per process and cached, so a malformed value fails the first request with the
// variable name instead of failing every request differently. Mirrors the fail-fast-at-boot rule
// SPEC-API §4 sets for app-serv.

import { env } from '$env/dynamic/private';
import { parseEnv, type PanelEnv } from '$lib/schemas/env';

let cached: PanelEnv | undefined;

export function panelEnv(): PanelEnv {
	cached ??= parseEnv(env);
	return cached;
}

// Test seam: lets a test reset the cache without reaching into module state.
export function resetPanelEnv(): void {
	cached = undefined;
}
