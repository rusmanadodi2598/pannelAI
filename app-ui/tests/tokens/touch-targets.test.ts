// Every checkbox label in the panel is drawn as a 44px row (docs/SPEC-UI/001-SPEC-UI.md §8.7).
//
// The label is the target, not the 16px box inside it: clicking the words toggles the checkbox, so a thumb
// has to be able to land on the words. jsdom runs no layout and a per-screen test only covers the screens a
// test renders, so this reads the markup of every component once. `state-scale.test.ts` measures the colour
// half of the same discipline; this is the size half.

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const SOURCE = resolve(process.cwd(), 'src');

function svelteFiles(dir: string): string[] {
	return readdirSync(dir).flatMap((entry) => {
		const path = join(dir, entry);
		if (statSync(path).isDirectory()) return svelteFiles(path);
		return entry.endsWith('.svelte') ? [path] : [];
	});
}

/** The class list of every `<label>` whose own block holds a checkbox, with the file it came from. */
function checkboxLabels(): { file: string; classes: string }[] {
	return svelteFiles(SOURCE).flatMap((file) =>
		Array.from(readFileSync(file, 'utf-8').matchAll(/<label([^>]*)>([\s\S]*?)<\/label>/g))
			.filter(([, , body]) => body.includes('type="checkbox"'))
			.map(([, attrs]) => ({
				file: file.slice(SOURCE.length + 1),
				classes: /class="([^"]*)"/.exec(String(attrs))?.[1] ?? ''
			}))
	);
}

describe('checkbox labels across the panel', () => {
	const labels = checkboxLabels();

	it('finds the labels this rule is written for, so an empty walk cannot pass silently', () => {
		// Six of these were 36px until 2026-10-20: two on the Settings security tab, one on its logging tab,
		// one each on the proxy form and the outbound card, and one on the one-time key modal.
		expect(labels.length).toBeGreaterThanOrEqual(15);
	});

	it('draws every one of them at least 44px tall', () => {
		const short = labels
			.filter((label) => !label.classes.includes('min-h-11'))
			.map((label) => `${label.file}: ${label.classes || '(no class)'}`);

		expect(short).toEqual([]);
	});
});
