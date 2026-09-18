// System calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.1.
//
// `/version` is public and answers with the build facts: version, commit, build date, Go version, and the
// embedded provider registry revision. The panel reads it to say which build it is talking to, which is
// what makes a changelog entry readable as "newer than what I run" instead of as a bare list.

import { z } from 'zod';
import { apiRequest, type ApiResult } from './client';

export const schemaSystemInfo = z.object({
	version: z.string(),
	commit: z.string(),
	build_date: z.string(),
	go_version: z.string(),
	registry_revision: z.string()
});

export type SystemInfo = z.infer<typeof schemaSystemInfo>;

export function fetchSystemInfo(): Promise<ApiResult<SystemInfo>> {
	return apiRequest<void, SystemInfo>({
		method: 'GET',
		path: '/version',
		schema: schemaSystemInfo
	});
}
