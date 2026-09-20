// Element readers for the panel's component tests.
//
// The panel's tests run on plain Vitest assertions, without a jest-dom matcher layer, so a control's
// state is read through the element rather than through a custom matcher. One definition, because
// two test files reading a checkbox two ways is how one of them starts asserting the wrong thing.

export function checked(element: HTMLElement): boolean {
	return (element as HTMLInputElement).checked;
}

export function value(element: HTMLElement): string {
	return (element as HTMLInputElement | HTMLSelectElement).value;
}

export function text(element: HTMLElement): string {
	return element.textContent ?? '';
}

/**
 * An element's text with whitespace runs collapsed to one space.
 *
 * Prose in a Svelte file is wrapped by the formatter, so a sentence can be split across a line break
 * and `textContent` then carries a newline and indentation inside it. An assertion about a sentence
 * needs the sentence, not the source's line wrapping, and a formatter pass must not be able to break
 * a test.
 */
export function squashed(element: HTMLElement): string {
	return (element.textContent ?? '').replace(/\s+/g, ' ').trim();
}
