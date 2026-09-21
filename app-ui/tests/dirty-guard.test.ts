// The unsaved-draft guard (docs/SPEC-UI/001-SPEC-UI.md §8.4.4).
//
// The spec's rule is one sentence ("Dirty state warns before leaving a form with unsaved changes"),
// and what makes it a rule rather than a habit is that the decision can be asserted without a browser
// dialog: the registry and the cancel decision are pure, so a fake `confirmLeave` stands in for the
// browser and the assertions are about what the guard decided, not about what jsdom can render.

import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	LEAVE_WARNING,
	hasDirtyForm,
	registerDirtyForm,
	shouldCancelNavigation
} from '../src/lib/dirty-guard';
import { forEachCase } from './support/tables';

// Every registration made by a test is removed again, because the registry is module state shared by
// the whole file and a leftover entry would make the next test's "nothing is dirty" assertion false.
const registrations: (() => void)[] = [];

function register(isDirty: () => boolean): void {
	registrations.push(registerDirtyForm(isDirty));
}

afterEach(() => {
	for (const unregister of registrations.splice(0)) unregister();
});

describe('dirty form registry', () => {
	it('reports no draft while nothing is registered', () => {
		expect(hasDirtyForm()).toBe(false);
	});

	it('reads the dirty state when a navigation happens, not when the form registers', () => {
		let dirty = false;
		register(() => dirty);

		expect(hasDirtyForm()).toBe(false);

		dirty = true;
		expect(hasDirtyForm()).toBe(true);

		// Saving or discarding is the form's own business; the registry follows it without re-registering.
		dirty = false;
		expect(hasDirtyForm()).toBe(false);
	});

	it('forgets a form when its unregister function runs', () => {
		const unregister = registerDirtyForm(() => true);
		expect(hasDirtyForm()).toBe(true);

		unregister();
		expect(hasDirtyForm()).toBe(false);
	});

	it('reports a draft while any one of several registered forms is dirty', () => {
		register(() => false);
		register(() => true);
		register(() => false);

		expect(hasDirtyForm()).toBe(true);
	});
});

describe('navigation decision', () => {
	const cases = [
		{
			name: 'lets a navigation through when nothing is dirty, without asking',
			dirty: false,
			willUnload: false,
			answer: false,
			cancelled: false,
			asked: false
		},
		{
			name: 'blocks an unload while a draft is dirty, and leaves the dialog to the browser',
			dirty: true,
			willUnload: true,
			answer: false,
			cancelled: true,
			asked: false
		},
		{
			name: 'asks before an in-panel navigation and cancels when the operator says no',
			dirty: true,
			willUnload: false,
			answer: false,
			cancelled: true,
			asked: true
		},
		{
			name: 'asks before an in-panel navigation and lets it through when the operator says yes',
			dirty: true,
			willUnload: false,
			answer: true,
			cancelled: false,
			asked: true
		}
	];

	forEachCase(cases, (testCase) => {
		if (testCase.dirty) register(() => true);

		const confirmLeave = vi.fn(() => testCase.answer);
		const cancelled = shouldCancelNavigation({ willUnload: testCase.willUnload }, confirmLeave);

		expect(cancelled).toBe(testCase.cancelled);
		expect(confirmLeave).toHaveBeenCalledTimes(testCase.asked ? 1 : 0);
	});

	it('asks with the sentence that names the loss', () => {
		register(() => true);

		const confirmLeave = vi.fn(() => true);
		shouldCancelNavigation({ willUnload: false }, confirmLeave);

		expect(confirmLeave).toHaveBeenCalledWith(LEAVE_WARNING);
		// The sentence is the panel's own prose: English, one line, and no em dash (R-02).
		expect(LEAVE_WARNING).not.toMatch(/\u2014/);
	});
});
