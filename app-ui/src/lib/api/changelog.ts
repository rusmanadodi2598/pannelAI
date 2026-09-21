// The served release notes, mirroring docs/SPEC-API/001-SPEC-API.md §7.18.
//
// One read: `GET /api/v1/changelog` through the panel's forwarder, so the session cookie is the only
// credential involved. The binary carries the history it was built from, which is why the panel renders
// what this route answers rather than a file bundled beside it: a second copy would drift from the
// running gateway and then lie about it (§6.12's reason, applied to release notes).

import { schemaChangelog, type Changelog } from '$lib/schemas/changelog';
import { apiRequest, type ApiResult } from './client';

/** The path the notes are served at, shown on the screen because that address is how it was read. */
export const CHANGELOG_PATH = '/changelog';

export function fetchChangelog(): Promise<ApiResult<Changelog>> {
	return apiRequest<void, Changelog>({
		method: 'GET',
		path: CHANGELOG_PATH,
		schema: schemaChangelog
	});
}
