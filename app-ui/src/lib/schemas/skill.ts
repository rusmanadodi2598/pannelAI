// The agent skill catalog, as the panel reads it (docs/SPEC-UI/001-SPEC-UI.md §6.10).
//
// One call: `GET /api/v1/skills` (SPEC-API §7.16). The gateway serves metadata and two URLs per
// capability and never a `SKILL.md` body, so this module declares the catalog and nothing about the
// documents themselves; what the panel derives from a row lives in `skill-source.ts`.
//
// Required fields are strict and additions are tolerated, per §7.4: the catalog belongs to the gateway,
// so a field added to a row must not break this screen.

import { z } from 'zod';
import { absoluteUrl } from './primitives';

export const schemaSkill = z.object({
	id: z.string(),
	name: z.string(),
	description: z.string(),
	/** The §7.15 route this skill teaches. Null for the entry skill, which indexes the others. */
	endpoint: z.string().nullable(),
	/** True for the one index row, which is the skill an operator pastes first. */
	entry: z.boolean(),
	/** The address an agent fetches, and the address the copy control hands out. */
	raw_url: absoluteUrl,
	/** The address a person reads, which renders the file rather than downloading it. */
	blob_url: absoluteUrl
});

export const schemaSkillCatalog = z.object({ data: z.array(schemaSkill) });

export type Skill = z.infer<typeof schemaSkill>;
export type SkillCatalog = z.infer<typeof schemaSkillCatalog>;
