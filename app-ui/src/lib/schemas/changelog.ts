// Changelog schemas, for the shape the gateway serves (docs/SPEC-UI/001-SPEC-UI.md §6.16).
//
// One call: `GET /api/v1/changelog` (SPEC-API §7.18). The binary carries the release notes it was built
// from, so the panel reads the history the running gateway reports and cannot drift from it. The route
// promises newest first and this module still sorts, because the order the screen depends on is the
// deterministic one: a same-day pair must not swap places between two reads.
//
// Two rules shape the fields:
//
//   `date` is a calendar date, not a timestamp. The contract declares `format: date` (`YYYY-MM-DD`), and
//   `Date.parse` alone is not that check: it accepts a non-ISO spelling such as `2026-9-2`, and it rolls a
//   day the month does not have (`2026-02-30`) into the next month. Both are refused here, so a value the
//   contract does not describe reads as drift rather than as a date the screen prints.
//
//   An entry's version is the string the gateway reports, kept verbatim. The panel compares versions to
//   mark what is newer than the running build, and it never rewrites the value it was given.

import { z } from 'zod';

const CALENDAR_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

/**
 * Whether a value is a real calendar date in the contract's `format: date`.
 *
 * The round trip is the second half of the check: a day the month does not have moves into the next
 * month, so the formatted result no longer equals what came in.
 */
function isCalendarDate(value: string): boolean {
	const match = CALENDAR_DATE.exec(value);
	if (match === null) return false;

	const [, year, month, day] = match;
	const parsed = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
	return parsed.toISOString().slice(0, 10) === value;
}

export const schemaChangelogEntry = z.object({
	version: z
		.string()
		.trim()
		.refine((value) => value.length > 0, { message: 'A release needs a version.' })
		.refine((value) => value.length <= 64, { message: 'Use 64 characters or fewer.' }),
	date: z.string().refine(isCalendarDate, { message: 'Invalid release date.' }),
	title: z
		.string()
		.trim()
		.refine((value) => value.length > 0, { message: 'A release needs a title.' }),
	// The note is the paragraph the gateway wrote, rendered as it stands. Required and not blank: an empty
	// note would render as a release that changed nothing, which is a claim the source did not make.
	notes: z
		.string()
		.trim()
		.refine((value) => value.length > 0, { message: 'A release needs a note.' })
});

export type ChangelogEntry = z.infer<typeof schemaChangelogEntry>;

export const schemaChangelog = z.object({
	data: z.array(schemaChangelogEntry)
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
 * Whether a running version can be compared to a release at all.
 *
 * This is what the screen's status line asks before it says anything about being behind: a gateway that
 * reports `dev-build` was read successfully, so "I could not read it" would be the wrong sentence, and a
 * count of zero newer releases is not "you are up to date" when nothing could be compared.
 */
export function canCompare(runningVersion: string): boolean {
	return runningVersion.trim().length > 0 && parseVersion(runningVersion) !== null;
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
 * A running version that cannot be compared returns `unknown` for every entry rather than marking
 * everything as newer, because "I could not place your version" is a different statement from "you are
 * behind on all of these", and the second one is confidently wrong in a way that would tell an operator to
 * upgrade something they already run.
 */
export function releaseMarker(entryVersion: string, runningVersion: string): ReleaseMarker {
	if (!canCompare(runningVersion)) return 'unknown';

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
		const byDate = Date.parse(right.date) - Date.parse(left.date);
		if (byDate !== 0) return byDate;
		return compareVersions(right.version, left.version);
	});
}

/** How many releases are newer than the running build, phrased for the screen's summary line. */
export function countNewer(entries: readonly ChangelogEntry[], runningVersion: string): number {
	if (!canCompare(runningVersion)) return 0;
	return entries.filter((entry) => releaseMarker(entry.version, runningVersion) === 'newer').length;
}
