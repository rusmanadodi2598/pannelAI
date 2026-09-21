// The agent skill catalog, and the reachability of the documents it points at
// (docs/SPEC-UI/001-SPEC-UI.md §6.10).
//
// Two reads with different rules. The catalog is the panel's own management API, so it goes through
// `apiRequest` with the session cookie and a schema. A document's address is a public file on the source
// host, so it is a plain request: no credential, no schema, and no panel API prefix.
//
// The source request is a HEAD, which asks whether the file is there without downloading it. The panel
// does not render a document's body, so fetching one would spend the operator's bandwidth to answer a
// question about a path.

import { apiRequest, type ApiResult } from './client';
import { schemaSkillCatalog, type SkillCatalog, type Skill } from '$lib/schemas/skill';
import {
	SOURCE_TIMEOUT_MS,
	classifySourceFailure,
	classifySourceStatus,
	type SourceProbe
} from '$lib/schemas/skill-source';

/** The path the catalog is served at, shown on the screen because that address is how it was read. */
export const SKILLS_PATH = '/skills';

export function fetchSkillCatalog(): Promise<ApiResult<SkillCatalog>> {
	return apiRequest<void, SkillCatalog>({
		method: 'GET',
		path: SKILLS_PATH,
		schema: schemaSkillCatalog
	});
}

/** Ask one document's address whether it is there. */
export async function probeSkillSource(url: string): Promise<SourceProbe> {
	try {
		const response = await fetch(url, {
			method: 'HEAD',
			signal: AbortSignal.timeout(SOURCE_TIMEOUT_MS),
			// A cached answer would report a state the operator may have just changed by publishing the file.
			cache: 'no-store'
		});

		return classifySourceStatus(response.status);
	} catch (error) {
		return classifySourceFailure(error);
	}
}

/** Ask every row's address, keyed by row id so the page can render one row at a time. */
export async function probeSkillSources(skills: Skill[]): Promise<Record<string, SourceProbe>> {
	const answered = await Promise.all(
		skills.map(async (skill) => [skill.id, await probeSkillSource(skill.raw_url)] as const)
	);

	return Object.fromEntries(answered);
}
