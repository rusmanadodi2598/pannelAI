// Every custom property the panel reads is declared somewhere it can be read from (DESIGN.md §3, §5).
//
// The proxy pass found two colours that pointed at nothing: `--color-primary-contrast` and
// `--color-surface-1` were used and never declared, and an undeclared var falls through silently instead of
// failing. This walks every declaration in `src/` (CSS files, and the inline style strings a component
// computes) and every `var(--name` use, and fails on a name with no declaration.

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const SOURCE = resolve(process.cwd(), 'src');

function sourceFiles(dir: string): string[] {
	return readdirSync(dir).flatMap((entry) => {
		const path = join(dir, entry);
		if (statSync(path).isDirectory()) return sourceFiles(path);
		return /\.(css|svelte|ts)$/.test(entry) ? [path] : [];
	});
}

const FILES = sourceFiles(SOURCE).map((file) => ({
	path: file.slice(SOURCE.length + 1),
	text: readFileSync(file, 'utf-8')
}));

function names(pattern: RegExp): Set<string> {
	const found = new Set<string>();
	for (const file of FILES) {
		for (const match of file.text.matchAll(pattern)) found.add(String(match[1]));
	}
	return found;
}

const DECLARED = names(/(?<![\w-])--([a-z0-9-]+)\s*:/g);
const USED = names(/var\(\s*--([a-z0-9-]+)/g);

describe('custom properties across the panel', () => {
	it('reads a real set of declarations and uses, so an empty walk cannot pass', () => {
		expect(DECLARED.size).toBeGreaterThan(30);
		expect(USED.size).toBeGreaterThan(20);
	});

	it('declares every property a component or stylesheet reads', () => {
		const undeclared = [...USED].filter((name) => !DECLARED.has(name)).sort();

		expect(
			undeclared,
			`tokens used but never declared (an undeclared var resolves to nothing):\n${undeclared.join('\n')}`
		).toEqual([]);
	});
});
