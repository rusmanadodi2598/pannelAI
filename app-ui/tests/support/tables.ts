// Table-driven test helper.
//
// docs/RULLES/TDD.md §2.5 requires every test function to run a table of input variations rather than
// a single scenario, so the loop lives here once instead of being rewritten per file. Each row becomes
// its own reported test, which keeps a failure naming the row that broke.

import { it } from 'vitest';

export function forEachCase<C extends { name: string }>(
	cases: C[],
	run: (testCase: C) => void
): void {
	for (const testCase of cases) {
		it(testCase.name, () => run(testCase));
	}
}
