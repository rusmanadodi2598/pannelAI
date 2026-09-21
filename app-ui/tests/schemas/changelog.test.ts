// Changelog derivations: how a release is compared, ordered, and marked against the running build.
//
// Table-driven per docs/RULLES/TDD.md §2.5. Version comparison is worth testing rather than reviewing,
// because it decides what the screen tells an operator about their own build and a wrong answer there is
// confidently wrong; and so is ordering, because the tie-break is what stops a same-day release pair from
// swapping places between two reads. The wire contract itself lives in `changelog-contract.test.ts`.

import { describe, expect, it } from 'vitest';
import {
	canCompare,
	compareVersions,
	countNewer,
	releaseMarker,
	sortChangelog,
	type ChangelogEntry
} from '$lib/schemas/changelog';

function entry(overrides: Partial<ChangelogEntry> = {}): ChangelogEntry {
	return {
		version: 'v1.0.0',
		date: '2026-01-01',
		title: 'A release',
		notes: 'Something changed.',
		...overrides
	};
}

describe('compareVersions', () => {
	const cases: { name: string; left: string; right: string; sign: number }[] = [
		{ name: 'equal with the same prefix style', left: 'v1.2.3', right: '1.2.3', sign: 0 },
		{ name: 'a major step up', left: '2.0.0', right: '1.9.9', sign: 1 },
		{ name: 'a patch step up', left: '1.2.4', right: '1.2.3', sign: 1 },
		{ name: 'a minor step down', left: '1.1.9', right: '1.2.0', sign: -1 },
		{ name: 'a missing segment counts as zero', left: '1.2', right: '1.2.0', sign: 0 },
		{
			name: 'a longer segment list wins on the extra value',
			left: '1.2.0.1',
			right: '1.2',
			sign: 1
		},
		{ name: 'zero against zero', left: '0.0.0', right: '0.0.0', sign: 0 },
		{ name: 'a large value', left: '1.2.999999999', right: '1.2.1', sign: 1 },
		{
			name: 'a prerelease is older than its release',
			left: '1.2.0-rc.1',
			right: '1.2.0',
			sign: -1
		},
		{ name: 'a release is newer than its prerelease', left: '1.2.0', right: '1.2.0-rc.1', sign: 1 },
		{
			name: 'two prereleases of the same version are equal here',
			left: '1.2.0-rc.1',
			right: '1.2.0-rc.2',
			sign: 0
		},
		{
			name: 'an unparsable value sorts below a version',
			left: 'nightly',
			right: '1.0.0',
			sign: -1
		},
		{ name: 'a version sorts above an unparsable value', left: '1.0.0', right: 'nightly', sign: 1 },
		{ name: 'empty against a version', left: '', right: '1.0.0', sign: -1 },
		{ name: 'a leading space is trimmed', left: ' v1.0.0', right: '1.0.0', sign: 0 }
	];

	for (const testCase of cases) {
		it(`${testCase.name}`, () => {
			const result = compareVersions(testCase.left, testCase.right);
			const sign = Math.sign(result);
			expect(sign, `${testCase.left} vs ${testCase.right} gave ${result}`).toBe(testCase.sign);
		});
	}
});

describe('canCompare', () => {
	const cases: { name: string; running: string; expected: boolean }[] = [
		{ name: 'a release number', running: '1.2.3', expected: true },
		{ name: 'a v-prefixed release number', running: 'v1.2.3', expected: true },
		{ name: 'a build number with a prerelease suffix', running: '0.1.0-dev', expected: true },
		{ name: 'an empty string', running: '', expected: false },
		{ name: 'whitespace only', running: '   ', expected: false },
		{ name: 'a word', running: 'nightly', expected: false },
		{ name: 'a word with a date in it', running: 'nightly-2026-09-20', expected: false }
	];

	for (const testCase of cases) {
		it(`${testCase.expected ? 'compares' : 'refuses'} ${testCase.name}`, () => {
			expect(canCompare(testCase.running)).toBe(testCase.expected);
		});
	}
});

describe('releaseMarker', () => {
	const cases: { name: string; entry: string; running: string; expected: string }[] = [
		{
			name: 'the entry is the running build',
			entry: 'v1.2.3',
			running: '1.2.3',
			expected: 'running'
		},
		{ name: 'the entry is ahead', entry: '1.3.0', running: '1.2.3', expected: 'newer' },
		{ name: 'the entry is behind', entry: '1.2.2', running: '1.2.3', expected: 'older' },
		{
			name: 'the running version could not be read',
			entry: '1.3.0',
			running: '',
			expected: 'unknown'
		},
		{
			name: 'the running version is whitespace',
			entry: '1.3.0',
			running: '   ',
			expected: 'unknown'
		},
		{
			name: 'the running version is unparsable',
			entry: '1.3.0',
			running: 'dev-build',
			expected: 'unknown'
		},
		{
			name: 'a prerelease is behind its release',
			entry: '1.2.0-rc.1',
			running: '1.2.0',
			expected: 'older'
		},
		{
			name: 'the running build is a prerelease of the entry',
			entry: '1.2.0',
			running: '1.2.0-rc.1',
			expected: 'newer'
		}
	];

	for (const testCase of cases) {
		it(`${testCase.name}`, () => {
			expect(releaseMarker(testCase.entry, testCase.running)).toBe(testCase.expected);
		});
	}

	it('never marks an entry as newer when the running version cannot be compared', () => {
		// The failure this guards: an unreadable or unparsable version making every entry look uninstalled,
		// which would tell an operator to upgrade to something they already run.
		const entries = [entry({ version: '9.9.9' }), entry({ version: '0.0.1' })];
		for (const running of ['', '   ', 'dev-build']) {
			for (const item of entries) {
				expect(releaseMarker(item.version, running)).not.toBe('newer');
			}
		}
	});
});

describe('sortChangelog', () => {
	it('orders newest first', () => {
		const sorted = sortChangelog([
			entry({ version: '1.0.0', date: '2026-01-01' }),
			entry({ version: '1.2.0', date: '2026-03-01' }),
			entry({ version: '1.1.0', date: '2026-02-01' })
		]);

		expect(sorted.map((item) => item.version)).toEqual(['1.2.0', '1.1.0', '1.0.0']);
	});

	it('breaks a same-day tie by version, whatever order the input arrives in', () => {
		const sameDay = '2026-01-01';
		const forward = sortChangelog([
			entry({ version: '1.0.0', date: sameDay }),
			entry({ version: '1.1.0', date: sameDay }),
			entry({ version: '1.2.0', date: sameDay })
		]);
		const reversed = sortChangelog([
			entry({ version: '1.2.0', date: sameDay }),
			entry({ version: '1.1.0', date: sameDay }),
			entry({ version: '1.0.0', date: sameDay })
		]);

		expect(forward.map((item) => item.version)).toEqual(['1.2.0', '1.1.0', '1.0.0']);
		expect(reversed.map((item) => item.version)).toEqual(forward.map((item) => item.version));
	});

	it('leaves the input array alone', () => {
		const input = [entry({ version: '1.0.0' }), entry({ version: '2.0.0' })];
		sortChangelog(input);
		expect(input.map((item) => item.version)).toEqual(['1.0.0', '2.0.0']);
	});

	it('returns an empty list for an empty list', () => {
		expect(sortChangelog([])).toEqual([]);
	});
});

describe('countNewer', () => {
	const entries = [
		entry({ version: '1.0.0' }),
		entry({ version: '1.1.0' }),
		entry({ version: '1.2.0' }),
		entry({ version: '2.0.0' })
	];

	const cases: { name: string; running: string; expected: number }[] = [
		{ name: 'behind by two', running: '1.1.0', expected: 2 },
		{ name: 'up to date', running: '2.0.0', expected: 0 },
		{ name: 'ahead of every release', running: '3.0.0', expected: 0 },
		{ name: 'an unknown running version', running: '', expected: 0 },
		{ name: 'an unparsable running version', running: 'nightly', expected: 0 }
	];

	for (const testCase of cases) {
		it(`counts ${testCase.expected} when ${testCase.name}`, () => {
			expect(countNewer(entries, testCase.running)).toBe(testCase.expected);
		});
	}
});
