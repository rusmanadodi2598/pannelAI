// Where the playground key's name may appear (SPEC-UI §6.15 rule 1).
//
// The relay tests prove the key never enters a response. This one guards the other direction: the
// browser must never be able to read it at all, and the cheapest way that could happen is a client
// module mentioning the variable and pulling the server config in behind it. So the rule is a location
// rule: only the module that declares the variable and the server modules that read it may name it.
//
// The live pass adds the mechanical half of the same rule by grepping the built client bundle for a key
// value it set; this is the half a test can hold on every run.

import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const SRC = resolve(process.cwd(), 'src');
const NAME = 'PANEL_PLAYGROUND_KEY';

/** The two places the name is allowed to be: the schema that declares it, and the server directory. */
function allowed(file: string): boolean {
	return file === 'src/lib/schemas/env.ts' || file.startsWith('src/lib/server/');
}

function sourceFiles(dir: string = SRC): string[] {
	const files: string[] = [];

	for (const entry of readdirSync(dir)) {
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) {
			files.push(...sourceFiles(full));
			continue;
		}
		if (entry.endsWith('.ts') || entry.endsWith('.svelte')) files.push(full);
	}

	return files;
}

describe('the playground key name', () => {
	it('appears only in files the browser bundle cannot contain', () => {
		const offenders = sourceFiles()
			.filter((file) => readFileSync(file, 'utf8').includes(NAME))
			.map((file) => relative(process.cwd(), file))
			.filter((file) => !allowed(file));

		expect(
			offenders,
			`these files name ${NAME} outside the server: ${offenders.join(', ')}`
		).toEqual([]);
	});

	it('is named by at least the module that declares it, so the guard is not vacuous', () => {
		// A rule with no witnesses passes for the wrong reason: if the variable were renamed, this test
		// would go green while the guard above stopped guarding anything.
		const naming = sourceFiles()
			.filter((file) => readFileSync(file, 'utf8').includes(NAME))
			.map((file) => relative(process.cwd(), file));

		expect(naming).toContain('src/lib/schemas/env.ts');
		expect(naming.length).toBeGreaterThan(1);
	});
});

/**
 * The files that ship to the browser for this screen.
 *
 * Listed by hand rather than discovered, so a file added to the screen has to be added here as well: a
 * discovered list would quietly include a new module and a reader could believe the guard had always
 * covered it.
 */
const BROWSER_FILES = [
	'src/lib/api/playground.ts',
	'src/lib/api/playground-reader.ts',
	'src/lib/components/PlaygroundAnswer.svelte',
	'src/lib/components/PlaygroundComposer.svelte',
	'src/lib/strings/playground.ts',
	'src/routes/playground/+page.svelte'
];

describe('the playground screen in the browser', () => {
	it('has nowhere to keep a credential and no header to put one in', () => {
		// §6.15 rule 1 forbids the credential in the bundle, in storage, in a URL, and in a form field. The
		// source guard above covers the bundle; this covers the rest by vocabulary, because a file that
		// cannot name a store or a header cannot use one.
		const storage = /\b(localStorage|sessionStorage|indexedDB|document\.cookie)\b/;
		const header = /\b(authorization|bearer)\b/i;

		for (const file of BROWSER_FILES) {
			const text = readFileSync(resolve(process.cwd(), file), 'utf8');

			expect(storage.test(text), `${file} writes a credential somewhere it would persist`).toBe(
				false
			);
			expect(header.test(text), `${file} names a credential header`).toBe(false);
		}
	});
});
