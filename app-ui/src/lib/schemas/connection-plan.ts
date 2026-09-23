// The pasted-list planner behind the provider detail screen's Add API Key dialog
// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// It is the panel's copy of the reference's planner (`src/shared/utils/bulkAdd.js` at 9router
// origin/master), and it exists for the reason that planner states: a connection's name is its identity
// inside its provider. The reference's backend upserts by name, so a colliding name overwrites instead
// of inserting; app-serv refuses one instead, through the unique index on (provider_id, label)
// (`app-serv/migrations/000005_upstream_endpoints.up.sql:29`). Either way a collision is not the row the
// operator asked for, so a generated name is gap-filled against the names already stored and against the
// names this same paste assigned.
//
// One deliberate difference from that planner: a line that *does* carry a name keeps it when it is free,
// because the reference's own copy promises the format `name|apiKey` while its code appends an index to
// every line (`bulkAdd.js:76-96`), a suffix that only exists to protect the upsert the panel does not
// have. A line without a name gets `Key <n>`, which is the reference's own base and the behaviour its
// copy calls "auto-named by index" (`AddApiKeyModal.js:206`).

/** One planned connection: the row's own line number, the name it will carry, and its key. */
export type PlannedConnection = { line: number; label: string; value: string };

/** The base a line without a name of its own gets, matching the reference (`bulkAdd.js:60`). */
const AUTO_NAME_BASE = 'Key';

/**
 * The smallest free name for `base`: `base <n>` from 1 up, or `base` itself where the caller allows the
 * bare form. Case-insensitive, like the reference's own comparison (`bulkAdd.js:80`).
 *
 * A generated name is never the bare base: a bare line has no name to reuse, so `Key` alone would be a
 * name the operator never chose and could not tell from the next one's.
 */
function freeName(base: string, used: Set<string>, bare: boolean): string {
	if (bare && !used.has(base.toLowerCase())) return base;

	for (let suffix = 1; ; suffix += 1) {
		const candidate = `${base} ${suffix}`;
		if (!used.has(candidate.toLowerCase())) return candidate;
	}
}

/**
 * Plans a paste: one connection per line, `name|key` or just `key`.
 *
 * The first bar separates the name and everything after it is the key, so a key that itself contains a
 * bar survives. Blank lines are skipped rather than reported, because a trailing newline is not a
 * mistake. Every row keeps the line it came from, so a refusal lands on the line the operator is looking
 * at instead of on a position among the good rows.
 */
export function planConnectionLines(
	text: string,
	existingLabels: readonly string[] = []
): PlannedConnection[] {
	const used = new Set(existingLabels.map((name) => name.toLowerCase()));
	const planned: PlannedConnection[] = [];

	text.split('\n').forEach((raw, index) => {
		const entry = raw.trim();
		if (entry === '') return;

		const bar = entry.indexOf('|');
		const typed = bar === -1 ? '' : entry.slice(0, bar).trim();
		const value = bar === -1 ? entry : entry.slice(bar + 1).trim();

		const label = freeName(typed === '' ? AUTO_NAME_BASE : typed, used, typed !== '');
		used.add(label.toLowerCase());

		planned.push({ line: index + 1, label, value });
	});

	return planned;
}
