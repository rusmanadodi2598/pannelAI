// A small stand-in for the served skill catalog, for the Skills tests.
//
// Not a copy of `app-serv/internal/handler/skills.go`: that catalog is the other app's data, and the live
// pass is what proves the panel parses the real rows. This fixture is the shape the panel reads, small
// enough to read in a diff, and it carries both cases the screen branches on: an entry row with a null
// endpoint, and capability rows that name one.

export type SkillRow = {
	id: string;
	name: string;
	description: string;
	endpoint: string | null;
	entry: boolean;
	raw_url: string;
	blob_url: string;
};

const RAW_BASE =
	'https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills';
const BLOB_BASE = 'https://github.com/rusmanadodi2598/pannelAI/blob/main/skills';

/** One catalog row, with the two addresses derived the way the handler derives them. */
export function skillRow(
	id: string,
	name: string,
	endpoint: string | null,
	entry = false
): SkillRow {
	return {
		id,
		name,
		description: `${name} through the gateway.`,
		endpoint,
		entry,
		raw_url: `${RAW_BASE}/${id}/SKILL.md`,
		blob_url: `${BLOB_BASE}/${id}/SKILL.md`
	};
}

/** The catalog in §7.16's shape: the entry skill first, then one row per capability. */
export function skillCatalog(rows: SkillRow[] = DEFAULT_ROWS): Record<string, unknown> {
	return { data: rows };
}

export const DEFAULT_ROWS: SkillRow[] = [
	skillRow('pannelai', 'pannelAI (Entry)', null, true),
	skillRow('pannelai-chat', 'Chat', '/chat/completions'),
	skillRow('pannelai-image', 'Image Generation', '/images/generations')
];
