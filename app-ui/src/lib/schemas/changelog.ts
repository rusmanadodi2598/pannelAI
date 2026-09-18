// Changelog schemas.
//
// The screen reads release notes for the running gateway. The data source is not decided yet
// (SPEC-UI §14 Q11): SPEC-API §7 defines no changelog endpoint, so the panel does not fetch. What is
// decided is the shape, and this file is that shape, so whichever source lands feeds the same contract
// and the rendering layer does not change.
//
// Two rules shape it:
//
//   `category` is an enum rather than a free string. A category the panel does not know is drift, which
//   R-38 and SPEC-UI §7.4 say to report rather than render.
//
//   An entry's version is the string the gateway reports, kept verbatim. The panel compares versions to
//   mark what is newer than the running build, and it never rewrites the value it was given.

import { z } from 'zod';

// The kinds of change a release carries. These are the category names this panel renders, and an unknown
// one fails the parse so the drift is visible instead of silently collapsing into "Other".
export const CHANGELOG_CATEGORIES = ['feature', 'fix', 'change', 'security', 'internal'] as const;

export const schemaChangelogEntry = z.object({
	version: z
		.string()
		.trim()
		.refine((value) => value.length > 0, { message: 'A release needs a version.' })
		.refine((value) => value.length <= 64, { message: 'Use 64 characters or fewer.' }),
	released_at: z
		.string()
		.refine((value) => !Number.isNaN(Date.parse(value)), { message: 'Invalid release date.' }),
	category: z.enum(CHANGELOG_CATEGORIES),
	// One line per entry. A release note is what changed, not a paragraph about it.
	items: z
		.array(
			z
				.string()
				.trim()
				.refine((value) => value.length > 0, { message: 'An entry cannot be empty.' })
		)
		.min(1, { message: 'A release needs at least one entry.' })
});

export type ChangelogEntry = z.infer<typeof schemaChangelogEntry>;

export const schemaChangelog = z.object({
	entries: z.array(schemaChangelogEntry)
});

export type Changelog = z.infer<typeof schemaChangelog>;

// Parses `v1.2.3`, `1.2.3`, and `1.2.3-rc.1` into comparable segments. A value that does not look like a
// version returns null, which sorts last and compares as unknown rather than as equal to zero.
function parseVersion(version: string): { segments: number[]; prerelease: boolean } | null {
	const match = version.trim().match(/^v?(\d+(?:\.\d+)*)(?:-([0-9A-Za-z.-]+))?$/);
	if (!match) return null;

	return {
		segments: match[1].split('.').map((part) => Number(part)),
		prerelease: match[2] !== undefined
	};
}

/**
 * Compares two version strings. Returns a negative number when `left` is older, positive when it is
 * newer, and 0 when they are equal.
 *
 * A version that cannot be parsed is treated as unknown: it sorts below every parseable version and never
 * compares equal to one, so an unrecognised value shows up as unknown rather than as "you are up to date".
 */
export function compareVersions(left: string, right: string): number {
	const a = parseVersion(left);
	const b = parseVersion(right);

	if (a === null && b === null) return 0;
	if (a === null) return -1;
	if (b === null) return 1;

	const length = Math.max(a.segments.length, b.segments.length);
	for (let index = 0; index < length; index += 1) {
		const diff = (a.segments[index] ?? 0) - (b.segments[index] ?? 0);
		if (diff !== 0) return diff;
	}

	// A prerelease is older than the release it leads to, which is what the semver rule says and what an
	// operator expects: `1.2.0-rc.1` is not the same as `1.2.0`.
	if (a.prerelease !== b.prerelease) return a.prerelease ? -1 : 1;

	return 0;
}

export type ReleaseMarker = 'running' | 'newer' | 'older' | 'unknown';

/**
 * Says how one release relates to the running build, which is the whole reason the screen exists: an
 * operator reading a changelog wants to know what changed since their version.
 *
 * `runningVersion` empty means the version endpoint did not answer. That returns `unknown` for every
 * entry rather than marking everything as newer, because "I could not read the running version" is a
 * different statement from "you are behind on all of these".
 */
export function releaseMarker(entryVersion: string, runningVersion: string): ReleaseMarker {
	if (runningVersion.trim().length === 0) return 'unknown';
	if (parseVersion(runningVersion) === null) return 'unknown';

	const order = compareVersions(entryVersion, runningVersion);
	if (order === 0) return 'running';
	return order > 0 ? 'newer' : 'older';
}

/**
 * Orders releases newest first, with the version as the tie-break.
 *
 * The tie-break is what makes the order deterministic: two releases on the same day must not swap places
 * between two reads, because a list that reorders itself for no reason reads as a bug.
 */
export function sortChangelog(entries: readonly ChangelogEntry[]): ChangelogEntry[] {
	return [...entries].sort((left, right) => {
		const byDate = Date.parse(right.released_at) - Date.parse(left.released_at);
		if (byDate !== 0) return byDate;
		return compareVersions(right.version, left.version);
	});
}

/** How many releases are newer than the running build, phrased for the screen's summary line. */
export function countNewer(entries: readonly ChangelogEntry[], runningVersion: string): number {
	if (runningVersion.trim().length === 0) return 0;
	return entries.filter((entry) => releaseMarker(entry.version, runningVersion) === 'newer').length;
}
